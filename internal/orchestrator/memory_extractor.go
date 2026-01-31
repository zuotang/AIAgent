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
		"scene": true, "state": true, "items": true,
		"emotion": true, "event": true, "relationship": true,
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
提取对话中的场景化信息，记录对话发生的情境和状态。

类型：
- scene: 场景信息（时间、地点、环境、天气、氛围）
- state: 状态信息（用户状态、助手状态、身体状况、心情）
- items: 物品信息（用户物品、助手物品、可用工具）
- emotion: 情绪信息（情绪基调、情感变化、亲密度）
- event: 事件信息（重要事件、活动、发生的事情）
- relationship: 关系信息（关系进展、互动方式、称呼变化）

各类型提取的 key 规范（key 名称不能包含 user、agent 字样）：
- scene: time (时间), location (地点), environment (环境), weather (天气), atmosphere (氛围), scenario (具体场景)
- state: physical_state (身体状态), mental_state (心理状态), mood (心情), energy_level (精力水平), activity (正在做的事)
- items: possession (拥有的物品), tool (使用的工具), equipment (装备), resource (资源)
- emotion: emotion_tone (情绪基调), feeling (感受), intimacy (亲密度), attitude (态度)
- event: important_event (重要事件), milestone (里程碑), activity_done (完成的活动), plan (计划)
- relationship: relationship_level (关系程度), interaction_style (互动方式), nickname (昵称/称呼), trust_level (信任度)

归属 (owner)：
- "user" - 用户相关的信息（用户的状态、物品、情绪等）
- "agent" - 助手相关的信息（助手的状态、物品、情绪等）
- 只提取明确表达的信息，不要推测或猜测

提取原则：
1. 只提取本轮对话中明确提到的信息
2. 记录具体的场景细节和状态变化
3. 不提取临时的、一次性的信息
4. 关注有助于理解对话情境的信息
5. 严禁把助手对用户的推测当成事实提取

【重要】只从"本轮对话"中提取，不要从"历史对话"中提取（历史对话仅供理解上下文）

输出格式：
{"memories":[{"type":"scene","key":"time","value":"晚上8点","confidence":1.0,"also_vector":true,"owner":"user","text":"对话发生在晚上8点"}]}

要求：
- key用英文小写+下划线
- confidence为0~1（确定性：1.0=明确提到，0.8=暗示，0.6=推测）
- also_vector=true表示写入向量库以便语义检索
- text是对该记忆的自然语言描述，用于向量检索`

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
	case "identity", "preference", "goal":
		// 旧的身份、偏好、目标类型迁移到关系类型
		m.Type = "relationship"
	case "context", "knowledge":
		// 旧的上下文、知识类型迁移到场景类型
		m.Type = "scene"
	case "tool", "constraint", "fact", "duration":
		m.Type = "scene"
	case "activity":
		m.Type = "event"
	}

	// 同义归一
	switch m.Type {
	case "state":
		if m.Key == "user_state" || m.Key == "state" {
			m.Key = "mental_state"
		}
		if m.Key == "mood" || m.Key == "feeling" {
			m.Key = "mood"
		}
	case "items":
		if m.Key == "user_items" || m.Key == "items" {
			m.Key = "possession"
		}
		if m.Key == "tools" || m.Key == "tool" {
			m.Key = "tool"
		}
	case "scene":
		if m.Key == "location" || m.Key == "place" {
			m.Key = "location"
		}
		if m.Key == "time" || m.Key == "datetime" {
			m.Key = "time"
		}
	}

	return m
}

// looksSensitive 检查是否为敏感信息
func looksSensitive(key, value string) bool {
	return false
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
