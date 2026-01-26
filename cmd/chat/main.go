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

	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/rag"
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
		"identity": true, "preference": true, "goal": true, "tool": true,
		"constraint": true, "fact": true, "activity": true, "duration": true,
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
func extractMemories(ctx context.Context, ollama *models.Client, recentTurns, userText, assistantText string, debug bool, extractorModel string) (ExtractorOutput, error) {
	sys := `你是"记忆提取器"。只输出严格 JSON，不要任何额外文字。

目标：从对话中提取未来能提升个性化与效率的长期信息。

重要归属规则（owner）：
- 从【用户输入】中提取的"关于用户自己的事实/偏好/长期目标" -> owner="user"
- 从【用户输入】中提取的"关于助手/Agent自己的设定" -> owner="agent"
- 从【助手回复】中提取的"关于助手自己设定（比如名字、风格）" -> owner="agent"
- 不要把【助手回复】里对用户的"推测/迎合/复述"当成用户事实写入记忆（除非用户本轮明确确认）。

规则：
- 只提取稳定、可复用的信息：偏好、身份信息、长期目标、常用工具/环境、重要约束、明确事实。
- 不要保存敏感信息（账号密码、身份证、精确住址等）。
- 不要保存短期一次性内容。
- 每条记忆必须包含：type,key,value,confidence,also_vector,owner，text 可选。
- key 必须是英文小写，用下划线分隔。
- type 必须是 identity/preference/goal/tool/constraint/fact/activity/duration。
- confidence 0~1；不确定就不要输出。
- also_vector=true 表示写入 Qdrant 作为可检索语义记忆。

输出格式：{"memories":[...]}（只输出JSON）`

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

	text, err := ollama.Chat(ctx, msgs, extractorModel)
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
	var (
		ollamaURL      = flag.String("ollama", "http://127.0.0.1:11434", "Ollama server URL")
		qdrantURL      = flag.String("qdrant", "http://127.0.0.1:6333", "Qdrant URL")
		collection     = flag.String("col", "memories", "Qdrant collection")
		chatModel      = flag.String("chat", "gemma3:12b", "Chat model")
		extractorModel = flag.String("extractor", "", "Extractor model (default: same as chat model)")
		embModel       = flag.String("embed", "nomic-embed-text", "Embedding model")
		topK           = flag.Int("k", 6, "TopK semantic recall")
		dbPath         = flag.String("db", "memory.db", "SQLite path")
		winN           = flag.Int("win", 8, "Short-term turns")
		debug          = flag.Bool("debug", false, "Enable debug output")
		timeoutSec     = flag.Int("timeout", 60, "Per-call timeout seconds (ollama/qdrant)")
	)
	flag.Parse()

	// 支持 Ctrl+C / kill 优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ollama := models.New(*ollamaURL, *chatModel, *embModel)
	ollama.SetDebug(*debug)

	// SQLite store
	mem, err := memory.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer mem.Close()

	// Qdrant store
	store := rag.NewStoreFromOllama(*qdrantURL, *collection, ollama)

	// 先做一次 embedding 拿维度，确保 collection 存在（加超时）
	{
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
		defer cancel()

		testVec, err := ollama.Embed(callCtx, "init")
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
	short := NewWindowMemory(*winN)
	fmt.Printf("AI Agent（user=%s，输入 exit 退出）\n", uid)

	// system：统一口径为“陪聊”，避免和“步骤/清单/示例”的专业要求打架
	system := `你是一个陪聊 Agent。
核心：
- 轻松陪伴、共情倾听、自然互动，主打闲聊解闷、情绪疏导，不做专业解答或长篇科普。
互动：
- 顺话题延展；没话题时主动抛轻量问题/小选择题；适配对方回复节奏，不敷衍不刷屏。
禁用：
- 不追问隐私、不聊敏感争议话题、不输出负能量、不强行主导话题。
原则：
- 先接情绪再回应；口语化不生硬；正向但不鸡汤。
- 你会参考“结构化长期记忆(SQLite)”和“语义长期记忆(Qdrant)”来保持一致性与个性化，但不要把记忆内容原样泄露给用户（除非用户要求你总结）。
- 【重要】长期记忆里：agent 表示你（assistant），user 表示用户本人，严禁混用。`

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

		// 1) 结构化长期记忆（SQLite）
		var structuredText string
		{
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
			structuredText, _ = mem.RenderStructuredMemory(callCtx, uid, 30)
			cancel()
		}

		// 2) 语义回忆（Qdrant，按 user_id 过滤）
		var recalledDocs []rag.Doc
		{
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
			recalledDocs, err = store.SimilaritySearch(callCtx, uid, userText, *topK)
			cancel()
			if err != nil {
				log.Fatal(err)
			}
		}

		var recalled strings.Builder
		recalled.WriteString("【语义长期记忆(Qdrant)】\n")
		// 控制回忆噪声：每条截断 + 最多保留 5 条
		maxRecall := 5
		for i, d := range recalledDocs {
			if i >= maxRecall {
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
		if *debug {
			red("\n发送给LLM的内容：\n")
			for _, msg := range msgs {
				red("%s: %s\n", msg.Role, msg.Content)
			}
			blue("\nLLM思考过程：正在生成回复...\n")
		}

		// 4) 生成回复（加超时）
		recentBeforeAdd := short.String() // ✅ 提取记忆用：避免本轮重复喂两份
		var assistantText string
		{
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
			assistantText, err = ollama.Chat(callCtx, msgs)
			cancel()
			if err != nil {
				log.Fatal(err)
			}
		}

		if *debug {
			blue("思考完成\n")
		}
		fmt.Println("\nAI：", assistantText)

		// 5) 自主学习：反思提取 -> 清洗校验 -> 写 SQLite/Qdrant（提取前不要 short.Add，避免重复上下文）
		extractorModelToUse := *extractorModel
		if extractorModelToUse == "" {
			extractorModelToUse = *chatModel
		}

		var extracted ExtractorOutput
		{
			callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
			extracted, err = extractMemories(callCtx, ollama, recentBeforeAdd, userText, assistantText, *debug, extractorModelToUse)
			cancel()
			if err != nil && *debug {
				log.Printf("提取记忆失败: %v", err)
			}
		}

		clean := sanitizeAndFilter(extracted.Memories)

		if *debug && len(extracted.Memories) > 0 {
			log.Printf("提取到 %d 条记忆，清洗后 %d 条", len(extracted.Memories), len(clean))
			for i, m := range clean {
				log.Printf("记忆 %d: type=%s, key=%s, value=%s, confidence=%.2f, also_vector=%t, owner=%s, text=%s",
					i+1, m.Type, m.Key, m.Value, m.Confidence, m.AlsoVector, m.Owner, m.Text)
			}
		}

		if len(clean) > 0 {
			// 写入 SQLite 结构化记忆（加超时）
			{
				callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
				if err := mem.UpsertExtractedMemories(callCtx, uid, clean); err != nil {
					if *debug {
						log.Printf("写入 SQLite 失败: %v", err)
					}
				} else if *debug {
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

			if *debug {
				log.Printf("准备写入 Qdrant 的语义记忆数量: %d", len(vectorTexts))
			}

			if len(vectorTexts) > 0 {
				callCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeoutSec)*time.Second)
				if err := store.UpsertTexts(callCtx, uid, vectorTexts); err != nil {
					if *debug {
						log.Printf("写入 Qdrant 失败: %v", err)
					}
				} else if *debug {
					log.Printf("成功写入 Qdrant 语义记忆")
				}
				cancel()
			}
		}

		// 6) 更新短期记忆（放到最后）
		short.Add(userText, assistantText)
	}
}
