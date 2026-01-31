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
	includeHistory bool,
) ([]memory.ExtractedMemory, error) {
	sys := `你是记忆提取器，只输出 JSON。
只提取稳定、长期有效、明确表达的信息
类型：
- identity: 身份（姓名、昵称、职业、年龄、技能）
- preference: 偏好（喜好、风格、习惯）
- goal: 目标（学习、职业、项目目标）
- context: 上下文（工具、环境、约束）
- knowledge: 知识（专业技能、经验）
各类型提取的 key 需遵循统一规范，key 名称不能包含 user、agent 字样：
- identity：name (姓名 / 昵称，通用)、career (职业)、age (年龄)、skill (技能)
- preference：hobby (喜好)、style (风格)、habit (习惯)
- goal：study_goal (学习目标)、career_goal (职业目标)、project_goal (项目目标)
- context：tool (工具)、env (环境)、constraint (约束)
- knowledge：prof_skill (专业技能)、experience (经验)
归属 (owner)：用户自主明确表达的信息→"user"；助手自身的设定信息→"agent"；仅提取 owner 为 user 的信息，不要提取 agent 信息，严禁把助手对用户的推测、猜测当成事实提取。

不提取：临时状态、一次性事件、推测信息

【重要】只从"本轮对话"中提取，不要从"历史对话"中提取（历史对话仅供理解上下文）

输出格式：
{"memories":[{"type":"identity","key":"name","value":"张三","confidence":1.0,"also_vector":true,"owner":"user","text":"用户名叫张三"}]}

要求：key用英文小写+下划线；confidence为0~1；also_vector=true表示写入向量库；text可选`

	var prompt string
	if includeHistory && recentTurns != "" {
		// 包含历史上下文（可能导致重复提取，但有助于理解）
		prompt = fmt.Sprintf(`历史对话（仅供理解上下文，不要从中提取）：
%s

===== 本轮对话（只从这里提取记忆）=====
用户: %s
助手: %s
==========================================

请输出 JSON：{"memories":[...]}（只输出JSON）`, recentTurns, userText, assistantText)
	} else {
		// 不包含历史上下文（避免重复提取，推荐）
		prompt = fmt.Sprintf(`本轮对话：
用户: %s
助手: %s

请输出 JSON：{"memories":[...]}（只输出JSON）`, userText, assistantText)
	}

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
