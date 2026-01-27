package main

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"agent-langchain/internal/config"
	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/rag"
	"agent-langchain/internal/tools"
)

// 彩色输出常量
const (
	ColorRed   = "\033[31m"
	ColorBlue  = "\033[34m"
	ColorReset = "\033[0m"
)

// 红色输出
func red(format string, args ...interface{}) {
	fmt.Printf(ColorRed+format+ColorReset, args...)
}

// 蓝色输出
func blue(format string, args ...interface{}) {
	fmt.Printf(ColorBlue+format+ColorReset, args...)
}

type ExtractorOutput struct {
	Memories []memory.ExtractedMemory `json:"memories"`
}

type Turn struct {
	User      string
	Assistant string
}

type ToolCall struct {
	ToolName  string
	Arguments string
}

// extractToolCall 从LLM响应中提取工具调用
// 格式: TOOL_CALL: tool_name("arguments")
func extractToolCall(response string) *ToolCall {
	// 查找 TOOL_CALL: 标记
	idx := strings.Index(response, "TOOL_CALL:")
	if idx == -1 {
		return nil
	}

	// 提取工具调用部分
	callPart := strings.TrimSpace(response[idx+len("TOOL_CALL:"):])

	// 解析工具名和参数
	// 格式: tool_name("arguments")
	openParen := strings.Index(callPart, "(")
	if openParen == -1 {
		return nil
	}

	toolName := strings.TrimSpace(callPart[:openParen])

	// 查找匹配的右括号
	closeParen := strings.LastIndex(callPart, ")")
	if closeParen == -1 || closeParen <= openParen {
		return nil
	}

	// 提取参数（去除引号）
	args := callPart[openParen+1 : closeParen]
	args = strings.Trim(args, `"'`)

	return &ToolCall{
		ToolName:  toolName,
		Arguments: args,
	}
}

// showMemoryStats 显示记忆访问统计
func showMemoryStats(cfg *config.Config, userID string) {
	// 初始化数据库
	mem, err := memory.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer mem.Close()

	// 获取统计信息
	stats, err := mem.GetTopAccessedMemories(context.Background(), userID, 20)
	if err != nil {
		log.Fatalf("获取统计信息失败: %v", err)
	}

	if len(stats) == 0 {
		fmt.Printf("用户 %s 还没有访问记录\n", userID)
		return
	}

	// 显示统计信息
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("记忆访问统计 - 用户: %s\n", userID)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	for i, s := range stats {
		fmt.Printf("%d. [%s/%s] %s = %s\n", i+1, s.Owner, s.Type, s.Key, s.Value)
		fmt.Printf("   访问次数: %d 次\n", s.AccessCount)
		fmt.Printf("   置信度: %.2f\n", s.Confidence)
		if s.LastAccessed != "" {
			fmt.Printf("   最后访问: %s\n", s.LastAccessed)
		}
		fmt.Printf("   更新时间: %s\n", s.UpdatedAt)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("共 %d 条记忆\n", len(stats))
}

type WindowMemory struct {
	N     int
	Turns []Turn
}

func NewWindowMemory(n int) *WindowMemory { return &WindowMemory{N: n} }

func (m *WindowMemory) Add(u, a string) {
	m.Turns = append(m.Turns, Turn{u, a})
	if len(m.Turns) > m.N {
		m.Turns = m.Turns[len(m.Turns)-m.N:]
	}
}

func (m *WindowMemory) String() string {
	var b strings.Builder
	for i, t := range m.Turns {
		b.WriteString(fmt.Sprintf("[TURN %d]\n", i+1))
		b.WriteString(fmt.Sprintf("[USER]\n%s\n", t.User))
		b.WriteString(fmt.Sprintf("[ASSISTANT]\n%s\n", t.Assistant))
	}
	return b.String()
}

var (
	validTypes = map[string]bool{
		"identity": true, "preference": true, "goal": true,
		"context": true, "knowledge": true,
	}
	validOwners = map[string]bool{"user": true, "agent": true}
	keyRe       = regexp.MustCompile(`^[a-z0-9_]+$`)
)

// normalize 负责归一化字段，避免 owner/type/key 混乱
func normalize(m memory.ExtractedMemory) memory.ExtractedMemory {
	m.Type = strings.TrimSpace(strings.ToLower(m.Type))
	m.Key = strings.TrimSpace(strings.ToLower(m.Key))
	m.Owner = strings.TrimSpace(strings.ToLower(m.Owner))
	m.Value = strings.TrimSpace(m.Value)
	m.Text = strings.TrimSpace(m.Text)

	// 旧类型迁移到新类型（兼容性处理）
	switch m.Type {
	case "tool", "constraint", "fact", "duration":
		m.Type = "context" // 工具、约束、事实、时长 -> 上下文信息
	case "activity":
		m.Type = "preference" // 活动习惯 -> 偏好
	}

	// 同义归一（按你 schema 扩展）
	switch m.Type {
	case "identity":
		// ⚠️ 注意括号：避免 || / && 优先级导致 owner 判断失效
		if m.Owner == "agent" && (m.Key == "assistant_name" || m.Key == "agent_name" || m.Key == "name") {
			m.Key = "name"
		}
		if m.Owner == "user" && (m.Key == "user_name" || m.Key == "name") {
			m.Key = "name"
		}
		if m.Key == "user_age" || m.Key == "age" {
			m.Key = "age"
		}
		if m.Key == "gender" {
			m.Key = "gender"
		}
	case "preference":
		if m.Key == "color_preference" || m.Key == "color" {
			m.Key = "color"
		}
	}

	return m
}

func looksSensitive(key, value string) bool {
	k := strings.ToLower(key)
	v := strings.ToLower(value)

	// 粗过滤：别把敏感信息写入长期记忆
	sensitiveKeys := []string{
		"password", "passwd", "pwd", "token", "secret", "api_key", "apikey",
		"id_card", "passport", "ssn", "address", "home_address",
		"phone", "mobile", "email", "bank", "credit", "card",
	}
	for _, sk := range sensitiveKeys {
		if strings.Contains(k, sk) {
			return true
		}
	}

	// 粗检测：疑似邮箱/手机号
	emailRe := regexp.MustCompile(`[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	phoneRe := regexp.MustCompile(`\b(\+?\d[\d\s\-]{7,}\d)\b`)
	if emailRe.MatchString(v) || phoneRe.MatchString(v) {
		return true
	}

	return false
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func fingerprint(owner, typ, key, val string) string {
	h := sha1.Sum([]byte(owner + "|" + typ + "|" + key + "|" + val))
	return hex.EncodeToString(h[:])
}

// deduplicateSemanticMemories 去除与结构化记忆重复的语义记忆
func deduplicateSemanticMemories(structuredText string, semanticDocs []rag.Doc) []rag.Doc {
	if structuredText == "" || len(semanticDocs) == 0 {
		return semanticDocs
	}

	// 提取结构化记忆中的关键词（简单实现：提取 value 部分）
	structuredKeywords := make(map[string]bool)
	lines := strings.Split(structuredText, "\n")
	for _, line := range lines {
		// 格式：- owner: type.key = value (conf=0.85)
		if strings.Contains(line, " = ") {
			parts := strings.Split(line, " = ")
			if len(parts) >= 2 {
				// 提取 value 部分（去除 confidence）
				value := strings.Split(parts[1], " (conf=")[0]
				value = strings.TrimSpace(strings.ToLower(value))
				if value != "" {
					structuredKeywords[value] = true
				}
			}
		}
	}

	// 过滤语义记忆
	result := make([]rag.Doc, 0, len(semanticDocs))
	for _, doc := range semanticDocs {
		content := strings.ToLower(doc.PageContent)
		isDuplicate := false

		// 检查是否与结构化记忆重复
		for keyword := range structuredKeywords {
			if len(keyword) > 3 && strings.Contains(content, keyword) {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			result = append(result, doc)
		}
	}

	return result
}

func sanitizeAndFilter(ms []memory.ExtractedMemory) []memory.ExtractedMemory {
	out := make([]memory.ExtractedMemory, 0, len(ms))
	seen := map[string]struct{}{}

	for _, m := range ms {
		m = normalize(m)

		if !validOwners[m.Owner] || !validTypes[m.Type] {
			continue
		}
		if m.Confidence < 0 || m.Confidence > 1 {
			continue
		}
		if !keyRe.MatchString(m.Key) {
			continue
		}
		if m.Value == "" || len(m.Value) > 512 {
			continue
		}
		if looksSensitive(m.Key, m.Value) {
			continue
		}

		fp := fingerprint(m.Owner, m.Type, m.Key, m.Value)
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = struct{}{}

		// 文本字段可选；不要太长
		m.Text = truncate(m.Text, 240)
		out = append(out, m)
	}

	return out
}

// 记忆提取器
func extractMemories(ctx context.Context, llm models.LLMClient, recentTurns, userText, assistantText string, debug bool, extractorModel string) (ExtractorOutput, error) {
	sys := `你是"记忆提取器"。只输出严格 JSON，不要任何额外文字。

目标：从对话中提取值得长期保存的信息。

【提取标准 - STABLE 原则】
只提取满足以下所有条件的信息：
1. Stable（稳定）：不会频繁变化
2. Timeless（时间无关）：不依赖特定时间点
3. Actionable（可操作）：未来可用于个性化
4. Broad（广泛适用）：在多个场景有用
5. Long-lasting（持久）：预期长期有效
6. Explicit（明确）：用户明确表达的

【记忆类型】
- identity: 身份信息（姓名、职业、年龄、技能等）
- preference: 偏好习惯（喜好、风格、习惯、活动）
- goal: 目标计划（学习目标、职业目标、项目目标）
- context: 上下文信息（工具、环境、约束、事实）
- knowledge: 知识技能（专业技能、经验、专长）

【归属规则 (owner)】
- 从【用户输入】中提取的"关于用户自己的信息" -> owner="user"
- 从【用户输入】中提取的"关于助手/Agent的设定" -> owner="agent"
- 从【助手回复】中提取的"关于助手自己的设定" -> owner="agent"
- 不要把【助手回复】里对用户的"推测/迎合/复述"当成用户事实（除非用户本轮明确确认）

【不要提取】
❌ 临时状态："今天很累"、"刚吃了饭"
❌ 一次性事件："昨天开会"、"刚到公司"
❌ 推测信息："可能喜欢"、"应该是"
❌ 敏感信息：密码、身份证、精确住址、手机号、邮箱

【输出格式】
{
  "memories": [
    {
      "type": "identity",
      "key": "name",
      "value": "张三",
      "confidence": 1.0,
      "also_vector": true,
      "owner": "user",
      "text": "用户名叫张三"
    }
  ]
}

要求：
- key 必须是英文小写，用下划线分隔
- confidence 范围 0~1，不确定就不要输出
- also_vector=true 表示写入向量数据库作为可检索语义记忆
- text 字段可选，用于提供更多上下文

只输出 JSON，不要任何额外文字。`

	// 注意：recentTurns 仅作为参考，本轮 user/assistant 单独传入，避免重复喂两份同样内容
	prompt := fmt.Sprintf(`最近对话（供参考，带标签 [USER]/[ASSISTANT] ）：
%s

本轮用户输入([USER])：
%s

本轮助手回复([ASSISTANT])：
%s

请输出 JSON：{"memories":[...]}（只输出JSON）`, recentTurns, userText, assistantText)

	msgs := []models.ChatMessage{
		{Role: "system", Content: sys},
		{Role: "user", Content: prompt},
	}

	text, err := llm.Chat(ctx, msgs, extractorModel)
	if err != nil {
		return ExtractorOutput{}, err
	}

	// 去除 Markdown 代码块标记
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
	}
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}
	text = strings.TrimSpace(text)

	if debug {
		log.Printf("extractor raw output: %s", text)
	}

	var out ExtractorOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return ExtractorOutput{}, fmt.Errorf("extractor json unmarshal failed: %w, raw=%s", err, text)
	}

	return out, nil
}

func main() {
	// 只保留配置文件路径参数
	configFile := flag.String("config", "config.deepseek.yaml", "配置文件路径")
	showStats := flag.Bool("stats", false, "显示记忆访问统计")
	statsUser := flag.String("user", "local", "查看统计的用户ID")
	flag.Parse()

	// 加载配置文件
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	// 如果是查看统计模式
	if *showStats {
		showMemoryStats(cfg, *statsUser)
		return
	}

	// 支持 Ctrl+C / kill 优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 创建 LLM 客户端（用于聊天和记忆提取）
	var llmClient models.LLMClient
	var chatModelName string
	switch cfg.Provider {
	case "deepseek":
		deepseek := models.NewDeepSeek(cfg.DeepSeek.BaseURL, cfg.DeepSeek.APIKey, cfg.DeepSeek.ChatModel)
		deepseek.SetDebug(cfg.Debug)
		llmClient = deepseek
		chatModelName = cfg.DeepSeek.ChatModel
		log.Printf("使用 DeepSeek API (base_url: %s, model: %s)", cfg.DeepSeek.BaseURL, chatModelName)
	case "ollama":
		ollama := models.New(cfg.Ollama.BaseURL, cfg.Ollama.ChatModel, cfg.Ollama.EmbedModel)
		ollama.SetDebug(cfg.Debug)
		llmClient = ollama
		chatModelName = cfg.Ollama.ChatModel
		log.Printf("使用 Ollama (model: %s)", chatModelName)
	default:
		log.Fatalf("Unknown provider: %s. Use 'ollama' or 'deepseek'", cfg.Provider)
	}

	// Embedding 始终使用 Ollama（DeepSeek 不支持 embedding）
	ollamaEmbed := models.New(cfg.Ollama.BaseURL, cfg.Ollama.ChatModel, cfg.Ollama.EmbedModel)
	ollamaEmbed.SetDebug(cfg.Debug)
	log.Printf("使用 Ollama 进行 Embedding (base_url: %s, model: %s)", cfg.Ollama.BaseURL, cfg.Ollama.EmbedModel)

	// SQLite store
	mem, err := memory.New(cfg.Database.Path)
	if err != nil {
		log.Fatal(err)
	}
	defer mem.Close()

	// Qdrant store（使用 Ollama 的 embedding）
	store := rag.NewStoreFromOllama(cfg.Qdrant.BaseURL, cfg.Qdrant.APIKey, cfg.Qdrant.Collection, ollamaEmbed)
	if cfg.Qdrant.APIKey != "" {
		log.Printf("使用 Qdrant (base_url: %s, 已启用 API Key 认证)", cfg.Qdrant.BaseURL)
	} else {
		log.Printf("使用 Qdrant (base_url: %s)", cfg.Qdrant.BaseURL)
	}

	// 先做一次 embedding 拿维度，确保 collection 存在（加超时）
	{
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
		defer cancel()

		testVec, err := ollamaEmbed.Embed(callCtx, "init")
		if err != nil {
			log.Fatalf("ollama embedding failed: %v", err)
		}
		if err := store.EnsureCollection(callCtx, len(testVec)); err != nil {
			log.Fatalf("ensure qdrant collection failed: %v", err)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("请输入你的 UID（第一行输入将作为用户ID）：")
	uid, _ := reader.ReadString('\n')
	uid = strings.TrimSpace(uid)
	if uid == "" {
		uid = "local"
	}

	// 短期记忆窗口
	short := NewWindowMemory(cfg.Memory.WindowSize)
	fmt.Printf("AI Agent（user=%s，输入 exit 退出）\n", uid)

	// system：统一口径为"陪聊"，避免和"步骤/清单/示例"的专业要求打架
	system := `你是一个陪聊 Agent，具备工具调用能力。
核心：
- 轻松陪伴、共情倾听、自然互动，主打闲聊解闷、情绪疏导，不做专业解答或长篇科普。
互动：
- 顺话题延展；没话题时主动抛轻量问题/小选择题；适配对方回复节奏，不敷衍不刷屏。
禁用：
- 不追问隐私、不聊敏感争议话题、不输出负能量、不强行主导话题。
原则：
- 先接情绪再回应；口语化不生硬；正向但不鸡汤。
- 你会参考"结构化长期记忆(SQLite)"和"语义长期记忆(Qdrant)"来保持一致性与个性化，但不要把记忆内容原样泄露给用户（除非用户要求你总结）。
- 【重要】长期记忆里：agent 表示你（assistant），user 表示用户本人，严禁混用。

【工具能力】
你可以使用以下工具来帮助用户：

1. **calculator** - 数学计算工具
   - 用途：进行数学计算（加减乘除、幂运算、平方根等）
   - 使用方式：当用户询问数学问题时，在回复中使用：TOOL_CALL: calculator("表达式")
   - 示例：
     * 用户："2的10次方是多少？" → 回复：TOOL_CALL: calculator("2^10")
     * 用户："计算 (5+3)*4" → 回复：TOOL_CALL: calculator("(5+3)*4")
     * 用户："16的平方根" → 回复：TOOL_CALL: calculator("sqrt(16)")
   - 支持的操作：+, -, *, /, ^(幂), sqrt(平方根), abs(绝对值)

【重要】当需要使用工具时：
1. 直接输出 TOOL_CALL: tool_name("参数")，不要添加其他文字
2. 工具执行后，你会收到结果，然后基于结果给用户自然的回复
3. 只在确实需要计算时使用工具，不要滥用`

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n收到退出信号，已结束。")
			return
		default:
		}

		fmt.Print("\n你：")
		userText, _ := reader.ReadString('\n')
		userText = strings.TrimSpace(userText)
		if userText == "" {
			continue
		}
		if userText == "exit" {
			return
		}

		// 1) 并行查询结构化记忆和语义记忆
		type memoryResult struct {
			structured string
			semantic   []rag.Doc
			err        error
		}

		structuredCh := make(chan memoryResult, 1)
		semanticCh := make(chan memoryResult, 1)

		// 并行查询 SQLite
		go func() {
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
			defer cancel()
			text, err := mem.RenderStructuredMemory(callCtx, uid, 30)
			structuredCh <- memoryResult{structured: text, err: err}
		}()

		// 并行查询 Qdrant
		go func() {
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
			defer cancel()
			docs, err := store.SimilaritySearch(callCtx, uid, userText, cfg.Qdrant.TopK)
			semanticCh <- memoryResult{semantic: docs, err: err}
		}()

		// 收集结果
		structuredResult := <-structuredCh
		semanticResult := <-semanticCh

		if structuredResult.err != nil && cfg.Debug {
			log.Printf("SQLite 查询失败: %v", structuredResult.err)
		}
		if semanticResult.err != nil {
			log.Fatal(semanticResult.err)
		}

		structuredText := structuredResult.structured
		recalledDocs := semanticResult.semantic

		// 去重：移除与结构化记忆重复的语义记忆
		recalledDocs = deduplicateSemanticMemories(structuredText, recalledDocs)

		var recalled strings.Builder
		recalled.WriteString("【语义长期记忆(Qdrant)】\n")
		// 使用配置的 top_k 值，不再硬编码
		for i, d := range recalledDocs {
			if i >= cfg.Qdrant.TopK {
				break
			}
			recalled.WriteString("- " + truncate(d.PageContent, 220) + "\n")
		}

		// 3) 组合 prompt：短期窗口 + 结构化 + 语义回忆
		prompt := fmt.Sprintf(`请基于以下信息回应用户（以陪聊为主）：

【短期对话窗口】（用于保持上下文）
%s

【结构化长期记忆(SQLite)】
%s

%s

【用户输入】
%s

输出要求：
- 先接情绪，再自然回应
- 给轻量建议/小选择题（不长篇科普）
- 如需追问，最多 2 个关键问题`,
			short.String(), structuredText, recalled.String(), userText)

		msgs := []models.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: prompt},
		}

		// debug: 红色输出发送内容，蓝色提示生成中
		if cfg.Debug {
			red("\n发送给LLM的内容：\n")
			for _, msg := range msgs {
				red("%s: %s\n", msg.Role, msg.Content)
			}
			blue("\nLLM思考过程：正在生成回复...\n")
		}

		// 4) 生成回复（使用流式响应）
		recentBeforeAdd := short.String() // ✅ 提取记忆用：避免本轮重复喂两份
		var assistantText string
		{
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
			defer cancel()

			// 尝试使用流式响应（仅Ollama支持）
			if ollamaClient, ok := llmClient.(*models.Client); ok {
				// 使用流式响应
				fmt.Print("\nAI：")
				tokenCh, errCh := ollamaClient.ChatStream(callCtx, msgs)

				var fullResponse strings.Builder
				for token := range tokenCh {
					fmt.Print(token)
					fullResponse.WriteString(token)
				}

				// 检查错误
				if err := <-errCh; err != nil {
					log.Fatal(err)
				}

				assistantText = fullResponse.String()
				fmt.Println() // 换行
			} else {
				// 非流式响应（DeepSeek等）
				assistantText, err = llmClient.Chat(callCtx, msgs)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println("\nAI：", assistantText)
			}
		}

		if cfg.Debug {
			blue("思考完成\n")
		}

		// 4.5) 检测并执行工具调用
		if strings.Contains(assistantText, "TOOL_CALL:") {
			// 提取工具调用
			toolCall := extractToolCall(assistantText)
			if toolCall != nil {
				if cfg.Debug {
					log.Printf("检测到工具调用: %s(%s)", toolCall.ToolName, toolCall.Arguments)
				}

				// 执行工具
				var toolResult string
				switch toolCall.ToolName {
				case "calculator":
					calc := &tools.CalculatorTool{}
					callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					result, err := calc.Execute(callCtx, toolCall.Arguments)
					cancel()
					if err != nil {
						toolResult = fmt.Sprintf("计算错误: %v", err)
					} else {
						toolResult = result
					}
				default:
					toolResult = fmt.Sprintf("未知工具: %s", toolCall.ToolName)
				}

				if cfg.Debug {
					log.Printf("工具执行结果: %s", toolResult)
				}

				// 将工具结果反馈给LLM，生成最终回复
				toolFeedbackPrompt := fmt.Sprintf(`工具执行结果：
工具: %s
输入: %s
输出: %s

请基于这个结果，用自然的语言回复用户。不要重复工具调用格式，直接给出友好的回答。`, toolCall.ToolName, toolCall.Arguments, toolResult)

				msgs = append(msgs, models.ChatMessage{
					Role:    "assistant",
					Content: assistantText,
				}, models.ChatMessage{
					Role:    "user",
					Content: toolFeedbackPrompt,
				})

				// 再次调用LLM生成最终回复（使用流式响应）
				callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)

				if ollamaClient, ok := llmClient.(*models.Client); ok {
					// 使用流式响应
					fmt.Print("\nAI：")
					tokenCh, errCh := ollamaClient.ChatStream(callCtx, msgs)

					var fullResponse strings.Builder
					for token := range tokenCh {
						fmt.Print(token)
						fullResponse.WriteString(token)
					}

					// 检查错误
					if err := <-errCh; err != nil {
						cancel()
						log.Fatal(err)
					}

					assistantText = fullResponse.String()
					fmt.Println() // 换行
				} else {
					// 非流式响应
					assistantText, err = llmClient.Chat(callCtx, msgs)
					if err != nil {
						cancel()
						log.Fatal(err)
					}
					fmt.Println("\nAI：", assistantText)
				}
				cancel()

				if cfg.Debug {
					blue("基于工具结果生成最终回复\n")
				}
			}
		} else {
			// 没有工具调用，直接显示回复（已在上面流式输出）
			// 不需要额外操作
		}

		// 5) 自主学习：反思提取 -> 清洗校验 -> 写 SQLite/Qdrant（提取前不要 short.Add，避免重复上下文）
		extractorModelToUse := cfg.Memory.ExtractorModel
		if extractorModelToUse == "" {
			extractorModelToUse = chatModelName
		}

		var extracted ExtractorOutput
		{
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
			extracted, err = extractMemories(callCtx, llmClient, recentBeforeAdd, userText, assistantText, cfg.Debug, extractorModelToUse)
			cancel()
			if err != nil && cfg.Debug {
				log.Printf("提取记忆失败: %v", err)
			}
		}

		clean := sanitizeAndFilter(extracted.Memories)

		if cfg.Debug && len(extracted.Memories) > 0 {
			log.Printf("提取到 %d 条记忆，清洗后 %d 条", len(extracted.Memories), len(clean))
			for i, m := range clean {
				log.Printf("记忆 %d: type=%s, key=%s, value=%s, confidence=%.2f, also_vector=%t, owner=%s, text=%s",
					i+1, m.Type, m.Key, m.Value, m.Confidence, m.AlsoVector, m.Owner, m.Text)
			}
		}

		if len(clean) > 0 {
			// 写入 SQLite 结构化记忆（加超时）
			{
				callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
				if err := mem.UpsertExtractedMemories(callCtx, uid, clean); err != nil {
					if cfg.Debug {
						log.Printf("写入 SQLite 失败: %v", err)
					}
				} else if cfg.Debug {
					log.Printf("成功写入 SQLite 结构化记忆")
				}
				cancel()
			}

			// 写入 Qdrant 语义记忆：短文本 + 去重 + 截断
			var vectorTexts []string
			seen := map[string]struct{}{}
			for _, m := range clean {
				if !m.AlsoVector || m.Confidence < 0.65 {
					continue
				}
				fp := fingerprint(m.Owner, m.Type, m.Key, m.Value)
				if _, ok := seen[fp]; ok {
					continue
				}
				seen[fp] = struct{}{}

				// 尽量短、稳定、利于召回
				vt := fmt.Sprintf("%s | %s:%s", m.Owner, m.Key, m.Value)
				if m.Text != "" {
					vt = fmt.Sprintf("%s | %s", m.Owner, m.Text)
				}
				vectorTexts = append(vectorTexts, truncate(vt, 240))
			}

			if cfg.Debug {
				log.Printf("准备写入 Qdrant 的语义记忆数量: %d", len(vectorTexts))
			}

			if len(vectorTexts) > 0 {
				callCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Timeout)*time.Second)
				if err := store.UpsertTexts(callCtx, uid, vectorTexts); err != nil {
					if cfg.Debug {
						log.Printf("写入 Qdrant 失败: %v", err)
					}
				} else if cfg.Debug {
					log.Printf("成功写入 Qdrant 语义记忆")
				}
				cancel()
			}
		}

		// 6) 更新短期记忆（放到最后）
		short.Add(userText, assistantText)
	}
}
