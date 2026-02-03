package orchestrator

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"agent-langchain/internal/models"
)

// classifyQueryType 使用小模型快速分类查询类型
// 返回: MEMORY, KNOWLEDGE, BOTH, NONE
func (o *orchestrator) classifyQueryType(ctx context.Context, userText string) string {
	// 过滤极短消息
	if len(strings.TrimSpace(userText)) < 2 {
		return "NONE"
	}

	// 创建超时上下文（使用分类器配置的超时时间）
	timeout := time.Duration(o.config.Classifier.Timeout) * time.Second
	classifyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 构建分类提示词
	prompt := fmt.Sprintf(`分析用户的查询，判断需要什么类型的上下文来回答。如果是一个听不懂的名词就需要调用知识库来回答。

上下文类型：
1. MEMORY（记忆）：用户的个人信息、偏好、历史对话、过往互动
   - 例如："我之前说过什么"、"你还记得我的名字吗"、"我喜欢什么"
2. KNOWLEDGE（知识库）：事实性信息、文档内容、技术知识、通用知识
   - 例如："什么是机器学习"、"如何使用这个API"、"解释一下原理"
3. BOTH（两者都需要）：既需要个人记忆又需要知识库
   - 例如："根据我的技能水平，推荐学习资料"
4. NONE（都不需要）：简单闲聊、问候、确认等
   - 例如："你好"、"谢谢"、"好的"

用户查询：%s

只回答以下之一：MEMORY、KNOWLEDGE、BOTH、NONE
回答：`, userText)

	msgs := []models.ChatMessage{
		{Role: "user", Content: prompt},
	}

	// 使用独立的分类器客户端
	classifierModel := o.config.Classifier.Model
	response, err := o.classifierClient.Chat(classifyCtx, msgs, classifierModel)
	if err != nil {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 查询分类失败: %v，默认使用 NONE\n", err)
		}
		return "NONE"
	}

	// 解析响应
	response = strings.TrimSpace(strings.ToUpper(response))

	// 提取分类结果
	if strings.Contains(response, "BOTH") {
		return "BOTH"
	} else if strings.Contains(response, "MEMORY") {
		return "MEMORY"
	} else if strings.Contains(response, "KNOWLEDGE") {
		return "KNOWLEDGE"
	} else if strings.Contains(response, "NONE") {
		return "NONE"
	}

	// 默认返回 NONE（保守策略）
	if o.config.Base.Debug {
		log.Printf("[DEBUG] 无法解析分类结果: %s，默认使用 NONE\n", response)
	}
	return "NONE"
}
