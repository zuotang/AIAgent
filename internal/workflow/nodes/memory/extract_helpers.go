package memory

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"agent-langchain/internal/workflow/registry"
)

var (
	validTypes  = map[string]bool{
		"scene": true, "state": true, "items": true,
		"emotion": true, "event": true, "relationship": true,
	}
	validOwners = map[string]bool{"user": true, "agent": true}
	keyRe       = regexp.MustCompile(`^[a-z0-9_]+$`)
)

func buildExtractorSystemPrompt() string {
	return `你是记忆提取器，只输出 JSON。
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

要求：
- key用英文小写+下划线
- confidence为0~1（确定性）
- importance为0~1（重要性，决定层级）
- layer为1/2/3（根据importance自动分配：>=0.8→1, 0.4-0.8→2, <0.4→3）
- also_vector=true表示写入向量库
- text是对该记忆的自然语言描述`
}

func buildExtractorUserPrompt(text, history string, includeHistory bool) string {
	if includeHistory && history != "" {
		return fmt.Sprintf(`历史对话（仅供理解上下文，不要从中提取）：
%s

===== 本轮对话（只从这里提取记忆）=====
%s
==========================================

请输出 JSON：{"memories":[...]}（只输出JSON）`, history, text)
	}
	return fmt.Sprintf(`本轮对话：
%s

请输出 JSON：{"memories":[...]}（只输出JSON）`, text)
}

func parseExtractorOutput(raw string) ([]registry.ExtractedMemoryItem, error) {
	text := strings.TrimSpace(raw)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
	}
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
	}
	text = strings.TrimSpace(text)

	// 修复常见 JSON 错误：缺少 "key": 前缀
	re := regexp.MustCompile(`("type"\s*:\s*"[^"]+"\s*,\s*)"([a-z_][a-z0-9_]*)"(\s*,\s*"value")`)
	text = re.ReplaceAllString(text, `$1"key":"$2"$3`)

	var out extractorOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w, raw=%s", err, text)
	}
	return out.Memories, nil
}

func sanitizeAndFilter(ms []registry.ExtractedMemoryItem) []registry.ExtractedMemoryItem {
	out := make([]registry.ExtractedMemoryItem, 0, len(ms))
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

		fp := fmt.Sprintf("%s:%s:%s:%s", m.Owner, m.Type, m.Key, m.Value)
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = struct{}{}

		if len(m.Text) > 240 {
			m.Text = m.Text[:240] + "..."
		}
		out = append(out, m)
	}
	return out
}

func normalize(m registry.ExtractedMemoryItem) registry.ExtractedMemoryItem {
	m.Type = strings.TrimSpace(strings.ToLower(m.Type))
	m.Key = strings.TrimSpace(strings.ToLower(m.Key))
	m.Owner = strings.TrimSpace(strings.ToLower(m.Owner))
	m.Value = strings.TrimSpace(m.Value)
	m.Text = strings.TrimSpace(m.Text)

	// 旧类型迁移
	switch m.Type {
	case "identity", "preference", "goal":
		m.Type = "relationship"
	case "context", "knowledge", "tool", "constraint", "fact", "duration":
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
	case "items":
		if m.Key == "user_items" || m.Key == "items" {
			m.Key = "possession"
		}
	case "scene":
		if m.Key == "place" {
			m.Key = "location"
		}
		if m.Key == "datetime" {
			m.Key = "time"
		}
	}

	// 根据 importance 自动分配 layer
	if m.Layer == 0 {
		if m.Importance >= 0.8 {
			m.Layer = 1
		} else if m.Importance >= 0.4 {
			m.Layer = 2
		} else {
			m.Layer = 3
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
