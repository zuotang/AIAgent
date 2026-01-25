package profile

import (
	"regexp"
	"strings"
)

// 正则：用户 & 智能体
var (
	// 用户相关正则
	reUserName = regexp.MustCompile(`(?:我叫|叫我|我是)([\p{Han}A-Za-z0-9_]{1,16})`)
	
	// 智能体相关正则
	reAgentName = regexp.MustCompile(`(?:你(?:以后)?叫|你的名字是)([\p{Han}A-Za-z0-9_]{1,16})`)
	reAgentPersonality = regexp.MustCompile(`你(的)?性格(是)?([\p{Han}A-Za-z0-9_]{1,24})`)
	
	// 通用信息正则
	reGender = regexp.MustCompile(`(?:性别[:：=]?\s*)?(女|男|女性|男性|中性|无性别)`)
	reAge = regexp.MustCompile(`(?:年龄[:：=]?\s*|^)(\d{1,3})(?:岁)?$`)
)

// IsMemoryCommand 检查文本是否是记忆命令
func IsMemoryCommand(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "请记住") ||
		strings.HasPrefix(text, "记住")
}

// ExtractAndUpdateProfile 提取并更新用户和智能体的个人资料
func ExtractAndUpdateProfile(userText string, profile map[string]any) (updated bool, notes []string) {
	// 确保profile是有效的map
	if profile == nil {
		profile = make(map[string]any)
	}
	
	// 分句处理
	clauses := strings.FieldsFunc(userText, func(r rune) bool {
		return r == '，' || r == ',' || r == '。' || r == ';' || r == '；'
	})
	
	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		
		// 检查是否是智能体相关语句
		isAgentClause := strings.Contains(clause, "你") || strings.Contains(clause, "你的")
		
		// 处理当前子句
		u, n := processClause(clause, profile, isAgentClause)
		if u {
			updated = true
			notes = append(notes, n...)
		}
	}
	
	return
}

// processClause 处理单个子句并更新profile
func processClause(clause string, profile map[string]any, isAgentClause bool) (updated bool, notes []string) {
	clause = strings.TrimSpace(strings.Trim(clause, "。.!！?？,，;；"))
	
	// 确定目标键
	targetKey := "user"
	if isAgentClause {
		targetKey = "agent"
	}
	
	// 确保目标对象存在
	if _, ok := profile[targetKey].(map[string]any); !ok {
		profile[targetKey] = make(map[string]any)
	}
	targetObj := profile[targetKey].(map[string]any)
	
	// 处理名字
	if updated, notes := processName(clause, targetObj, isAgentClause); updated {
		return updated, notes
	}
	
	// 预处理：去掉前缀，方便匹配
	trimmed := clause
	trimmed = strings.TrimPrefix(trimmed, "你是")
	trimmed = strings.TrimPrefix(trimmed, "我是")
	trimmed = strings.TrimPrefix(trimmed, "你今年")
	trimmed = strings.TrimPrefix(trimmed, "我今年")
	trimmed = strings.TrimPrefix(trimmed, "你")
	trimmed = strings.TrimPrefix(trimmed, "我")
	trimmed = strings.TrimSpace(trimmed)
	
	// 处理性别
	if updated, notes := processGender(trimmed, targetObj, isAgentClause); updated {
		return updated, notes
	}
	
	// 处理年龄
	if updated, notes := processAge(trimmed, targetObj, isAgentClause); updated {
		return updated, notes
	}
	
	// 处理智能体性格
	if isAgentClause {
		if updated, notes := processPersonality(clause, targetObj); updated {
			return updated, notes
		}
	}
	
	return false, nil
}

// processName 处理名字信息
func processName(clause string, targetObj map[string]any, isAgentClause bool) (updated bool, notes []string) {
	if isAgentClause {
		// 智能体名字
		if m := regexp.MustCompile(`你(?:以后)?叫([\p{Han}A-Za-z0-9_]{1,16})`).FindStringSubmatch(clause); len(m) >= 2 {
			targetObj["name"] = m[1]
			return true, []string{"好的，我以后就叫「" + m[1] + "」。"}
		}
		if m := regexp.MustCompile(`你的名字是([\p{Han}A-Za-z0-9_]{1,16})`).FindStringSubmatch(clause); len(m) >= 2 {
			targetObj["name"] = m[1]
			return true, []string{"我的名字是「" + m[1] + "」。"}
		}
	} else {
		// 用户名字
		if m := regexp.MustCompile(`我叫([\p{Han}A-Za-z0-9_]{1,16})`).FindStringSubmatch(clause); len(m) >= 2 {
			targetObj["name"] = m[1]
			return true, []string{"好的，我记住你叫「" + m[1] + "」。"}
		}
		if m := regexp.MustCompile(`叫我([\p{Han}A-Za-z0-9_]{1,16})`).FindStringSubmatch(clause); len(m) >= 2 {
			targetObj["name"] = m[1]
			return true, []string{"好的，我记住你叫「" + m[1] + "」。"}
		}
		if m := regexp.MustCompile(`^我是([\p{Han}A-Za-z0-9_]{1,16})$`).FindStringSubmatch(clause); len(m) >= 2 {
			targetObj["name"] = m[1]
			return true, []string{"好的，我记住你叫「" + m[1] + "」。"}
		}
	}
	return false, nil
}

// processGender 处理性别信息
func processGender(clause string, targetObj map[string]any, isAgentClause bool) (updated bool, notes []string) {
	if m := reGender.FindStringSubmatch(clause); len(m) >= 2 {
		gender := m[1]
		switch gender {
		case "女", "女性":
			targetObj["gender"] = "female"
		case "男", "男性":
			targetObj["gender"] = "male"
		default:
			targetObj["gender"] = "neutral"
		}
		
		if isAgentClause {
			return true, []string{"好的，我的性别已设置。"}
		}
		return true, []string{"好的，我记住你的性别信息了。"}
	}
	return false, nil
}

// processAge 处理年龄信息
func processAge(clause string, targetObj map[string]any, isAgentClause bool) (updated bool, notes []string) {
	if m := reAge.FindStringSubmatch(clause); len(m) >= 2 {
		age := m[1]
		targetObj["age"] = age
		
		if isAgentClause {
			return true, []string{"好的，我的年龄设为 " + age + "。"}
		}
		return true, []string{"好的，我记住你的年龄了。"}
	}
	return false, nil
}

// processPersonality 处理智能体性格信息
func processPersonality(clause string, targetObj map[string]any) (updated bool, notes []string) {
	if m := reAgentPersonality.FindStringSubmatch(clause); len(m) >= 2 {
		personality := strings.TrimSpace(m[1])
		targetObj["personality"] = personality
		return true, []string{"明白，我会以「" + personality + "」的性格回应。"}
	}
	
	// 支持自然表达
	switch {
	case strings.Contains(clause, "温柔点") || strings.Contains(clause, "温柔一点"):
		targetObj["personality"] = "gentle"
		return true, []string{"好，我会更温柔一些。"}
	case strings.Contains(clause, "理性点") || strings.Contains(clause, "更理性"):
		targetObj["personality"] = "rational"
		return true, []string{"好，我会更理性、更结构化。"}
	case strings.Contains(clause, "幽默点") || strings.Contains(clause, "搞笑点"):
		targetObj["personality"] = "humorous"
		return true, []string{"好，我会更幽默一点。"}
	}
	
	return false, nil
}
