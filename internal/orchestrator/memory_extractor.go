package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
)

// ExtractorOutput 记忆提取器输出
type ExtractorOutput struct {
	Memories []memory.ExtractedMemory `json:"memories"`
}

var (
	validTypes = map[string]bool{
		"identity": true, "preference": true, "goal": true,
		"context": true, "knowledge": true,
	}
	validOwners = map[string]bool{"user": true, "agent": true}
	keyRe       = regexp.MustCompile(`^[a-z0-9_]+$`)
)

// ExtractMemories 从对话中提取记忆
func ExtractMemories(
	ctx context.Context,
	llm models.LLMClient,
	recentTurns string,
	userText string,
	assistantText string,
	debug bool,
	extractorModel string,
) ([]memory.ExtractedMemory, error) {
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
		return nil, err
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
		fmt.Printf("extractor raw output: %s\n", text)
	}

	var out ExtractorOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("extractor json unmarshal failed: %w, raw=%s", err, text)
	}

	// 清洗和过滤
	cleaned := sanitizeAndFilter(out.Memories)

	if debug && len(out.Memories) > 0 {
		fmt.Printf("提取到 %d 条记忆，清洗后 %d 条\n", len(out.Memories), len(cleaned))
		for i, m := range cleaned {
			fmt.Printf("记忆 %d: type=%s, key=%s, value=%s, confidence=%.2f, also_vector=%t, owner=%s, text=%s\n",
				i+1, m.Type, m.Key, m.Value, m.Confidence, m.AlsoVector, m.Owner, m.Text)
		}
	}

	return cleaned, nil
}

// normalize 归一化记忆字段
func normalize(m memory.ExtractedMemory) memory.ExtractedMemory {
	m.Type = strings.TrimSpace(strings.ToLower(m.Type))
	m.Key = strings.TrimSpace(strings.ToLower(m.Key))
	m.Owner = strings.TrimSpace(strings.ToLower(m.Owner))
	m.Value = strings.TrimSpace(m.Value)
	m.Text = strings.TrimSpace(m.Text)

	// 旧类型迁移到新类型
	switch m.Type {
	case "tool", "constraint", "fact", "duration":
		m.Type = "context"
	case "activity":
		m.Type = "preference"
	}

	// 同义归一
	switch m.Type {
	case "identity":
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

// looksSensitive 检查是否为敏感信息
func looksSensitive(key, value string) bool {
	k := strings.ToLower(key)
	v := strings.ToLower(value)

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

	// 检测邮箱/手机号
	emailRe := regexp.MustCompile(`[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	phoneRe := regexp.MustCompile(`\b(\+?\d[\d\s\-]{7,}\d)\b`)
	if emailRe.MatchString(v) || phoneRe.MatchString(v) {
		return true
	}

	return false
}

// sanitizeAndFilter 清洗和过滤记忆
func sanitizeAndFilter(ms []memory.ExtractedMemory) []memory.ExtractedMemory {
	out := make([]memory.ExtractedMemory, 0, len(ms))
	seen := make(map[string]struct{})

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

		// 截断文本字段
		m.Text = truncate(m.Text, 240)
		out = append(out, m)
	}

	return out
}
