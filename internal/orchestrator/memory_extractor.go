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
提取对话中的场景化信息，并根据重要性和持久性分配记忆层级。

【三层记忆架构】
1. 硬记忆（Layer 1）- 永久存储，核心信息
   - 身份信息：姓名、年龄、性别、职业
   - 核心偏好：重要的喜好、价值观
   - 关键事实：不会改变的信息
   - 特点：高重要性(0.8-1.0)，永久保存

2. 中等记忆（Layer 2）- 半永久，上下文信息
   - 近期事件：最近发生的事情
   - 对话主题：讨论过的话题
   - 情感状态：情绪、关系进展
   - 特点：中等重要性(0.4-0.8)，可能过期

3. 软记忆（Layer 3）- 临时，当前会话
   - 当前场景：时间、地点、环境
   - 临时状态：当前心情、正在做的事
   - 一次性信息：不需要长期保存的内容
   - 特点：低重要性(0.0-0.4)，会话结束后清除

类型：
- scene: 场景信息（时间、地点、环境、天气、氛围）
- state: 状态信息（用户状态、助手状态、身体状况、心情）
- items: 物品信息（用户物品、助手物品、可用工具）
- emotion: 情绪信息（情绪基调、情感变化、亲密度）
- event: 事件信息（重要事件、活动、发生的事情）
- relationship: 关系信息（关系进展、互动方式、称呼变化）

各类型提取的 key 规范（key 名称不能包含 user、agent 字样）：
- scene: time, location, environment, weather, atmosphere, scenario
- state: physical_state, mental_state, mood, energy_level, activity
- items: possession, tool, equipment, resource
- emotion: emotion_tone, feeling, intimacy, attitude
- event: important_event, milestone, activity_done, plan
- relationship: relationship_level, interaction_style, nickname, trust_level

归属 (owner)：
- "user" - 用户相关的信息
- "agent" - 助手相关的信息

【重要性评分规则】
- 1.0: 核心身份信息（姓名、年龄等）→ Layer 1
- 0.8-0.9: 重要偏好、关键事实 → Layer 1
- 0.6-0.7: 重要事件、主题讨论 → Layer 2
- 0.4-0.5: 一般事件、情绪状态 → Layer 2
- 0.2-0.3: 临时场景、当前状态 → Layer 3
- 0.0-0.1: 一次性信息 → Layer 3

提取原则：
1. 只提取本轮对话中明确提到的信息
2. 根据信息的持久性和重要性分配层级
3. 核心身份信息必须标记为 Layer 1
4. 临时场景信息标记为 Layer 3
5. 严禁把助手对用户的推测当成事实提取

【重要】只从"本轮对话"中提取，不要从"历史对话"中提取（历史对话仅供理解上下文）

输出格式（严格遵守JSON格式）：
{"memories":[{"type":"relationship","key":"nickname","value":"小明","confidence":1.0,"also_vector":true,"owner":"user","text":"用户的昵称是小明","layer":1,"importance":0.9}]}

【JSON格式要求】：
- 每个记忆对象必须包含所有必需字段：type, key, value, confidence, also_vector, owner, text, layer, importance
- key字段必须写成 "key":"字段名" 的格式，不能省略 "key":
- 所有字符串值必须用双引号包围
- 数字值不需要引号
- 布尔值使用 true/false（小写，无引号）

要求：
- key用英文小写+下划线
- confidence为0~1（确定性）
- importance为0~1（重要性，决定层级）
- layer为1/2/3（根据importance自动分配：>=0.8→1, 0.4-0.8→2, <0.4→3）
- also_vector=true表示写入向量库
- text是对该记忆的自然语言描述`

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

	// 修复常见的JSON格式错误
	text = fixCommonJSONErrors(text)

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

// fixCommonJSONErrors 修复LLM生成的常见JSON格式错误
func fixCommonJSONErrors(text string) string {
	// 修复缺少 "key": 的情况
	// 例如: {"type":"state","user_age","value":18} -> {"type":"state","key":"user_age","value":18}
	// 匹配模式: "type":"xxx","<字段名>","value"
	re := regexp.MustCompile(`("type"\s*:\s*"[^"]+"\s*,\s*)"([a-z_][a-z0-9_]*)"(\s*,\s*"value")`)
	text = re.ReplaceAllString(text, `$1"key":"$2"$3`)

	return text
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

	// 根据 importance 自动分配 layer（如果未指定）
	if m.Layer == 0 {
		if m.Importance >= 0.8 {
			m.Layer = 1 // 硬记忆
		} else if m.Importance >= 0.4 {
			m.Layer = 2 // 中等记忆
		} else {
			m.Layer = 3 // 软记忆
		}
	}

	// 如果 importance 未指定，根据 layer 设置默认值
	if m.Importance == 0 {
		switch m.Layer {
		case 1:
			m.Importance = 0.9
		case 2:
			m.Importance = 0.6
		case 3:
			m.Importance = 0.3
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

// fingerprint 生成记忆指纹用于去重
func fingerprint(owner, typ, key, val string) string {
	return fmt.Sprintf("%s:%s:%s:%s", owner, typ, key, val)
}

// truncate 截断字符串到指定长度
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
