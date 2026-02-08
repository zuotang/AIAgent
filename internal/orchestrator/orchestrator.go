package orchestrator

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"agent-langchain/internal/agent"
	"agent-langchain/internal/config"
	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/rag"
	"agent-langchain/internal/utils"
)

// Orchestrator 编排器接口
type Orchestrator interface {
	ProcessMessage(ctx context.Context, userID string, agentID uint, userText string, conversationHistory string, conversationMessages []models.ChatMessage, systemPrompt string) (agent.Output, error)
	ProcessMessageStream(ctx context.Context, userID string, agentID uint, userText string, conversationHistory string, conversationMessages []models.ChatMessage, systemPrompt string, callback func(string) error) (agent.Output, error)
	GetConfig() *config.Config
	GetStore() *memory.Store
	GetVectorStore() *rag.QdrantStore
	GetChatHistory(ctx context.Context, userID string, agentID uint, limit, offset int) ([]memory.ChatMessage, error)
	GetChatHistoryWithCursor(ctx context.Context, userID string, agentID uint, beforeID uint, limit int) ([]memory.ChatMessage, error)
	GetChatHistoryCount(ctx context.Context, userID string, agentID uint) (int64, error)
	GetChatHistoryAfterID(ctx context.Context, userID string, agentID uint, afterID uint, limit int) ([]memory.ChatMessage, error)
	GetChatSessions(ctx context.Context, userID string, agentID uint) ([]memory.ChatSession, error)
}

// orchestrator 编排器，负责协调各个组件
type orchestrator struct {
	config           *config.Config
	llmClient        models.LLMClient
	classifierClient models.LLMClient // 独立的分类器客户端
	extractorClient  models.LLMClient // 独立的记忆提取器客户端
	ollamaClient     *models.Client
	memStore         *memory.Store
	vectorStore      *rag.QdrantStore
	agent            agent.Agent
	chatModel        string
}

// New 创建新的编排器
func New(
	cfg *config.Config,
	llmClient models.LLMClient,
	classifierClient models.LLMClient,
	extractorClient models.LLMClient,
	ollamaClient *models.Client,
	memStore *memory.Store,
	vectorStore *rag.QdrantStore,
	ag agent.Agent,
	chatModel string,
) Orchestrator {
	return &orchestrator{
		config:           cfg,
		llmClient:        llmClient,
		classifierClient: classifierClient,
		extractorClient:  extractorClient,
		ollamaClient:     ollamaClient,
		memStore:         memStore,
		vectorStore:      vectorStore,
		agent:            ag,
		chatModel:        chatModel,
	}
}

// GetConfig 获取配置
func (o *orchestrator) GetConfig() *config.Config {
	return o.config
}

// GetStore 获取存储实例
func (o *orchestrator) GetStore() *memory.Store {
	return o.memStore
}

// GetVectorStore 获取向量存储实例
func (o *orchestrator) GetVectorStore() *rag.QdrantStore {
	return o.vectorStore
}

// GetChatHistory 获取聊天记录
func (o *orchestrator) GetChatHistory(ctx context.Context, userID string, agentID uint, limit, offset int) ([]memory.ChatMessage, error) {
	return o.memStore.GetChatHistory(ctx, userID, agentID, limit, offset)
}

// GetChatHistoryWithCursor 获取聊天记录（基于游标的分页）
func (o *orchestrator) GetChatHistoryWithCursor(ctx context.Context, userID string, agentID uint, beforeID uint, limit int) ([]memory.ChatMessage, error) {
	return o.memStore.GetChatHistoryWithCursor(ctx, userID, agentID, beforeID, limit)
}

// GetChatHistoryCount 获取聊天记录总数
func (o *orchestrator) GetChatHistoryCount(ctx context.Context, userID string, agentID uint) (int64, error) {
	return o.memStore.GetChatHistoryCount(ctx, userID, agentID)
}

// GetChatHistoryAfterID 获取指定消息ID之后的聊天记录
func (o *orchestrator) GetChatHistoryAfterID(ctx context.Context, userID string, agentID uint, afterID uint, limit int) ([]memory.ChatMessage, error) {
	return o.memStore.GetChatHistoryAfterID(ctx, userID, agentID, afterID, limit)
}

// GetChatSessions 获取聊天会话列表
func (o *orchestrator) GetChatSessions(ctx context.Context, userID string, agentID uint) ([]memory.ChatSession, error) {
	return o.memStore.GetChatSessions(ctx, userID, agentID)
}

// ProcessMessage 处理用户消息
func (o *orchestrator) ProcessMessage(
	ctx context.Context,
	userID string,
	agentID uint,
	userText string,
	conversationHistory string,
	conversationMessages []models.ChatMessage,
	systemPrompt string,
) (agent.Output, error) {
	// 1. 智能路由：使用小模型快速分类查询类型
	queryType := "NONE"
	if o.config.Knowledge.EnableRouting {
		queryType = o.classifyQueryType(ctx, userText)
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 查询分类结果: %s", queryType)
		}
	}

	// 2. 检索并合并上下文（使用提取的公共函数）
	contextResult, err := o.retrieveAndMergeContext(ctx, userID, agentID, userText, queryType)
	if err != nil {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 上下文检索失败: %v", err)
		}
	}

	// 3. 合并上下文
	var contextParts []string
	if contextResult.MemoryText != "" {
		contextParts = append(contextParts, contextResult.MemoryText)
	}
	if contextResult.KnowledgeText != "" {
		contextParts = append(contextParts, contextResult.KnowledgeText)
	}
	combinedContext := strings.Join(contextParts, "\n\n")

	// 4. 显示上下文统计（debug模式）
	// if o.config.Base.Debug {
	// 	o.showContextStats(systemPrompt, structuredText, semanticDocs, conversationHistory, userText)
	// }

	// 5. 不进行上下文压缩，直接使用原始对话历史
	compressedContext := conversationHistory

	// 6. 读取对话摘要（若存在且落后最近窗口）
	var conversationSummary string
	cutoffID := o.getRecentWindowCutoffID(ctx, userID, agentID, 20)
	if compressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID); err == nil && compressed.CompressedText != "" {
		if cutoffID == 0 || compressed.LastMessageID < cutoffID {
			conversationSummary = compressed.CompressedText
		}
	}

	// 7. 构建Agent输入
	input := agent.Input{
		UserID:               userID,
		Message:              userText,
		SystemPrompt:         systemPrompt,
		Memory:               combinedContext, // 合并的记忆和知识库上下文
		Conversation:         compressedContext,
		ConversationMessages: conversationMessages,
		ConversationSummary:  conversationSummary,
	}

	// 7. 执行Agent
	output, err := o.agent.Run(ctx, input)
	if err != nil {
		return agent.Output{}, err
	}

	// 保存聊天记录并获取消息ID
	sessionID := fmt.Sprintf("%s_%d", userID, time.Now().Unix())
	_, err = o.memStore.SaveChatMessage(ctx, userID, "user", userText, sessionID, agentID)
	if err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 保存用户消息失败: %v", err)
	}

	// 移除thinking标签后再保存助手响应
	cleanedResponse := removeThinkingTags(output.Response)
	cleanedResponse = stripParenthetical(cleanedResponse)
	assistantMsgID, err := o.memStore.SaveChatMessage(ctx, userID, "assistant", cleanedResponse, sessionID, agentID)
	if err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 保存助手响应失败: %v", err)
	}

	// 异步更新对话摘要（落后最近20轮，最大600字）
	if assistantMsgID > 0 {
		go func() {
			// 使用 background context 避免父 context 取消影响异步任务
			// 设置超时防止 goroutine 泄漏
			bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
			defer cancel()

			if err := o.updateConversationSummary(bgCtx, userID, agentID, 20, 600); err != nil && o.config.Base.Debug {
				log.Printf("[DEBUG] 异步更新对话摘要失败: %v", err)
			}
		}()
	}

	// 5. 异步提取并存储记忆（智能触发）
	if o.config.Memory.EnableExtractor {
		// 将判断和提取都放到异步执行，避免 LLM 分类器阻塞主流程
		go func() {
			// 使用 background context 避免父 context 取消影响异步任务
			// 设置超时防止 goroutine 泄漏
			bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
			defer cancel()

			// 异步判断是否需要提取记忆
			if o.shouldExtractMemory(userText, output.Response) {
				if err := o.extractAndStoreMemories(bgCtx, userID, agentID, conversationHistory, userText, output.Response); err != nil {
					if o.config.Base.Debug {
						log.Printf("[DEBUG] 异步提取记忆失败: %v", err)
					}
				}
			} else if o.config.Base.Debug {
				log.Printf("[DEBUG] 跳过记忆提取：未触发条件")
			}
		}()
	} else if o.config.Base.Debug {
		log.Printf("[DEBUG] 记忆提取功能已禁用 (memory.enable_extractor=false)")
	}

	return output, nil
}

// ProcessMessageStream 流式处理用户消息
func (o *orchestrator) ProcessMessageStream(
	ctx context.Context,
	userID string,
	agentID uint,
	userText string,
	conversationHistory string,
	conversationMessages []models.ChatMessage,
	systemPrompt string,
	callback func(string) error,
) (agent.Output, error) {
	// 1. 智能路由：使用小模型快速分类查询类型
	queryType := "NONE"
	if o.config.Knowledge.EnableRouting {
		queryType = o.classifyQueryType(ctx, userText)
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 流式处理 - 查询分类结果: %s", queryType)
		}
	}

	// 2. 检索并合并上下文（使用提取的公共函数）
	contextResult, err := o.retrieveAndMergeContext(ctx, userID, agentID, userText, queryType)
	if err != nil {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 流式处理 - 上下文检索失败: %v", err)
		}
	}

	// 3. 合并上下文
	var contextParts []string
	if contextResult.MemoryText != "" {
		contextParts = append(contextParts, contextResult.MemoryText)
	}
	if contextResult.KnowledgeText != "" {
		contextParts = append(contextParts, contextResult.KnowledgeText)
	}
	combinedContext := strings.Join(contextParts, "\n\n")

	// 4. 显示上下文统计（debug模式）
	// if o.config.Base.Debug {
	// 	o.showContextStats(systemPrompt, structuredText, semanticDocs, conversationHistory, userText)
	// }

	// 5. 不进行上下文压缩，直接使用原始对话历史
	compressedContext := conversationHistory

	// 6. 读取对话摘要（若存在且落后最近窗口）
	var conversationSummary string
	cutoffID := o.getRecentWindowCutoffID(ctx, userID, agentID, 20)
	if compressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID); err == nil && compressed.CompressedText != "" {
		if cutoffID == 0 || compressed.LastMessageID < cutoffID {
			conversationSummary = compressed.CompressedText
		}
	}

	// 7. 构建Agent输入
	input := agent.Input{
		UserID:               userID,
		Message:              userText,
		SystemPrompt:         systemPrompt,
		Memory:               combinedContext, // 合并的记忆和知识库上下文
		Conversation:         compressedContext,
		ConversationMessages: conversationMessages,
		ConversationSummary:  conversationSummary,
	}

	// 7. 执行Agent流式处理
	output, err := o.agent.RunStream(ctx, input, callback)
	if err != nil {
		return agent.Output{}, err
	}

	// 5. 保存聊天记录并获取消息ID
	sessionID := fmt.Sprintf("%s_%d", userID, time.Now().Unix())
	_, err = o.memStore.SaveChatMessage(ctx, userID, "user", userText, sessionID, agentID)
	if err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 保存用户消息失败: %v", err)
	}

	// 移除thinking标签后再保存助手响应
	cleanedResponseStream := removeThinkingTags(output.Response)
	cleanedResponseStream = stripParenthetical(cleanedResponseStream)
	assistantMsgID, err := o.memStore.SaveChatMessage(ctx, userID, "assistant", cleanedResponseStream, sessionID, agentID)
	if err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 保存助手响应失败: %v", err)
	}

	// 异步更新对话摘要（落后最近20轮，最大600字）
	if assistantMsgID > 0 {
		go func() {
			// 使用 background context 避免父 context 取消影响异步任务
			// 设置超时防止 goroutine 泄漏
			bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
			defer cancel()

			if err := o.updateConversationSummary(bgCtx, userID, agentID, 20, 600); err != nil && o.config.Base.Debug {
				log.Printf("[DEBUG] 流式处理 - 异步更新对话摘要失败: %v", err)
			}
		}()
	}

	// 6. 异步提取并存储记忆（智能触发）
	if o.config.Memory.EnableExtractor {
		// 将判断和提取都放到异步执行，避免 LLM 分类器阻塞主流程
		go func() {
			// 使用 background context 避免父 context 取消影响异步任务
			// 设置超时防止 goroutine 泄漏
			bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
			defer cancel()

			// 异步判断是否需要提取记忆
			if o.shouldExtractMemory(userText, output.Response) {
				if err := o.extractAndStoreMemories(bgCtx, userID, agentID, conversationHistory, userText, output.Response); err != nil {
					if o.config.Base.Debug {
						log.Printf("[DEBUG] 异步提取记忆失败: %v", err)
					}
				}
			} else if o.config.Base.Debug {
				log.Printf("[DEBUG] 跳过记忆提取：未触发条件")
			}
		}()
	} else if o.config.Base.Debug {
		log.Printf("[DEBUG] 记忆提取功能已禁用 (memory.enable_extractor=false)")
	}

	return output, nil
}

// getRecentWindowCutoffID 返回“最近N轮”窗口中最早消息的ID
// 若消息不足则返回0。
func (o *orchestrator) getRecentWindowCutoffID(ctx context.Context, userID string, agentID uint, turns int) uint {
	if turns <= 0 {
		return 0
	}
	limit := turns * 2
	msgs, err := o.memStore.GetChatHistoryWithCursor(ctx, userID, agentID, 0, limit)
	if err != nil || len(msgs) == 0 {
		return 0
	}
	// msgs为ID倒序，最后一个为最早
	return msgs[len(msgs)-1].ID
}

// updateConversationSummary 更新对话摘要，确保摘要落后最近N轮
func (o *orchestrator) updateConversationSummary(ctx context.Context, userID string, agentID uint, turns int, maxLen int) error {
	cutoffID := o.getRecentWindowCutoffID(ctx, userID, agentID, turns)
	if cutoffID == 0 {
		return nil
	}

	compressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID)
	var lastID uint
	if err == nil && compressed != nil {
		lastID = compressed.LastMessageID
	}
	// 如果摘要已覆盖最近窗口，需重建摘要（防止摘要与最近N轮重复）
	if lastID >= cutoffID {
		if err := o.memStore.ClearCompressedContext(ctx, userID, agentID); err != nil {
			return err
		}
		allMsgs, err := o.memStore.GetChatHistoryBetweenIDs(ctx, userID, agentID, 0, cutoffID, 500)
		if err != nil {
			return err
		}
		newConversation, newLastID := buildConversationText(allMsgs)
		if newConversation == "" || newLastID == 0 {
			return nil
		}
		compressorModel := o.config.Extractor.Model
		if compressorModel == "" {
			compressorModel = o.chatModel
		}
		_, err = CompressContextIncremental(ctx, o.extractorClient, o.memStore, userID, agentID, newConversation, newLastID, compressorModel, maxLen)
		return err
	}
	// 已经追到最近窗口之前，无需更新
	if lastID >= cutoffID-1 {
		return nil
	}

	// 读取待摘要的历史消息（ID在(lastID, cutoffID)）
	var batch []memory.ChatMessage
	nextAfterID := lastID
	for {
		msgs, err := o.memStore.GetChatHistoryBetweenIDs(ctx, userID, agentID, nextAfterID, cutoffID, 200)
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			break
		}
		batch = append(batch, msgs...)
		nextAfterID = msgs[len(msgs)-1].ID
		if nextAfterID >= cutoffID-1 {
			break
		}
	}

	newConversation, newLastID := buildConversationText(batch)
	if newConversation == "" || newLastID == 0 {
		return nil
	}

	compressorModel := o.config.Extractor.Model
	if compressorModel == "" {
		compressorModel = o.chatModel
	}

	_, err = CompressContextIncremental(ctx, o.extractorClient, o.memStore, userID, agentID, newConversation, newLastID, compressorModel, maxLen)
	return err
}

func buildConversationText(msgs []memory.ChatMessage) (string, uint) {
	if len(msgs) == 0 {
		return "", 0
	}
	var b strings.Builder
	var lastID uint
	for i := 0; i < len(msgs)-1; i++ {
		userMsg := msgs[i]
		assistantMsg := msgs[i+1]
		if userMsg.Role != "user" || assistantMsg.Role != "assistant" {
			continue
		}
		userText := utils.PreprocessLite(userMsg.Content)
		assistantText := utils.PreprocessLite(assistantMsg.Content)
		if userText == "" || assistantText == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("User: ")
		b.WriteString(userText)
		b.WriteString("\n")
		b.WriteString("Assistant: ")
		b.WriteString(assistantText)
		lastID = assistantMsg.ID
		i++
	}
	return strings.TrimSpace(b.String()), lastID
}

func stripParenthetical(text string) string {
	if text == "" {
		return text
	}
	// Remove full-width and half-width parenthetical content.
	re := regexp.MustCompile(`\([^)]*\)|（[^）]*）`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}

// extractAndStoreMemories 提取并存储记忆
func (o *orchestrator) extractAndStoreMemories(
	ctx context.Context,
	userID string,
	agentID uint,
	conversationHistory string,
	userText string,
	assistantText string,
) error {
	// 使用记忆提取器模型
	extractorModel := o.config.Extractor.Model
	println("====记忆提取模型", extractorModel)
	if extractorModel == "" {
		extractorModel = o.chatModel
	}

	// 提取记忆
	memories, err := ExtractMemories(
		ctx,
		o.extractorClient, // 使用独立的记忆提取器客户端
		conversationHistory,
		userText,
		assistantText,
		o.config.Base.Debug,
		extractorModel,
		o.config.Memory.IncludeHistoryContext,
	)
	if err != nil {
		return err
	}

	if len(memories) == 0 {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] ========== 记忆提取结果 ==========")
			log.Printf("[DEBUG] 未提取到任何记忆")
			log.Printf("[DEBUG] ===================================")
		}
		return nil
	}

	// 打印提取的记忆详细内容
	if o.config.Base.Debug {
		log.Printf("[DEBUG] ========== 记忆提取结果 ==========")
		log.Printf("[DEBUG] 提取到 %d 条记忆:", len(memories))
		for i, m := range memories {
			log.Printf("[DEBUG] 记忆 %d:", i+1)
			log.Printf("[DEBUG]   类型(type): %s", m.Type)
			log.Printf("[DEBUG]   归属(owner): %s", m.Owner)
			log.Printf("[DEBUG]   键(key): %s", m.Key)
			log.Printf("[DEBUG]   值(value): %s", m.Value)
			log.Printf("[DEBUG]   置信度(confidence): %.2f", m.Confidence)
			log.Printf("[DEBUG]   重要性(importance): %.2f", m.Importance)
			log.Printf("[DEBUG]   层级(layer): %d", m.Layer)
			log.Printf("[DEBUG]   写入向量库(also_vector): %t", m.AlsoVector)
			log.Printf("[DEBUG]   描述(text): %s", m.Text)
			log.Printf("[DEBUG]   ---")
		}
		log.Printf("[DEBUG] ===================================")
	}

	// 存储到 SQLite
	if err := o.storeStructuredMemories(ctx, userID, agentID, memories); err != nil {
		return err
	}

	// 存储到 Qdrant
	if err := o.storeSemanticMemories(ctx, userID, agentID, memories); err != nil {
		return err
	}

	return nil
}

// storeStructuredMemories 存储结构化记忆
func (o *orchestrator) storeStructuredMemories(
	ctx context.Context,
	userID string,
	agentID uint,
	memories []memory.ExtractedMemory,
) error {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Base.Timeout)*time.Second)
	defer cancel()

	if err := o.memStore.UpsertExtractedMemories(callCtx, userID, agentID, memories); err != nil {
		if o.config.Base.Debug {
			log.Printf("写入 SQLite 失败: %v\n", err)
		}
		return err
	}

	if o.config.Base.Debug {
		log.Printf("成功写入 SQLite 结构化记忆\n")
	}
	return nil
}

// storeSemanticMemories 存储语义记忆
func (o *orchestrator) storeSemanticMemories(
	ctx context.Context,
	userID string,
	agentID uint,
	memories []memory.ExtractedMemory,
) error {
	var vectorTexts []string
	seen := make(map[string]struct{})

	for _, m := range memories {
		if !m.AlsoVector || m.Confidence < o.config.Memory.MinConfidence {
			continue
		}

		fp := fingerprint(m.Owner, m.Type, m.Key, m.Value)
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = struct{}{}

		vt := fmt.Sprintf("%s | %s:%s", m.Owner, m.Key, m.Value)
		if m.Text != "" {
			vt = fmt.Sprintf("%s | %s", m.Owner, m.Text)
		}
		vectorTexts = append(vectorTexts, truncate(vt, 240))
	}

	if len(vectorTexts) == 0 {
		return nil
	}

	if o.config.Base.Debug {
		log.Printf("准备写入 Qdrant 的语义记忆数量: %d\n", len(vectorTexts))
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Base.Timeout)*time.Second)
	defer cancel()

	if err := o.vectorStore.UpsertMemoryTexts(callCtx, userID, agentID, vectorTexts); err != nil {
		if o.config.Base.Debug {
			log.Printf("写入 Qdrant 失败: %v\n", err)
		}
		return err
	}

	if o.config.Base.Debug {
		log.Printf("成功写入 Qdrant 语义记忆\n")
	}
	return nil
}

// shouldExtractMemory 判断是否需要提取记忆
func (o *orchestrator) shouldExtractMemory(userText, assistantText string) bool {
	// 如果未启用智能触发，总是提取
	if !o.config.Memory.EnableSmartTrigger {
		return true
	}

	// 预过滤：过滤简单应答
	if o.isSimpleResponse(userText) {
		return false
	}

	// 根据配置的触发方法选择策略
	switch o.config.Memory.TriggerMethod {
	case "keyword":
		return o.shouldExtractByKeyword(userText, assistantText)
	case "llm":
		return o.shouldExtractByLLM(userText, assistantText)
	case "conservative":
		return o.shouldExtractConservative(userText, assistantText)
	default:
		// 默认使用保守策略
		return o.shouldExtractConservative(userText, assistantText)
	}
}

// shouldLoadMemory 判断是否需要按需加载记忆（剧情触发）
// isSimpleResponse 检查是否为简单应答
func (o *orchestrator) isSimpleResponse(userText string) bool {
	simpleResponses := []string{
		"好的", "好", "嗯", "哦", "啊", "呃",
		"谢谢", "谢了", "多谢", "感谢",
		"ok", "okay", "yes", "no", "yeah",
		"哈哈", "呵呵", "嘿嘿", "嘻嘻",
		"👌", "👍", "😊", "😄",
	}
	userLower := strings.TrimSpace(strings.ToLower(userText))
	for _, resp := range simpleResponses {
		if userLower == resp {
			return true
		}
	}
	return false
}

// shouldExtractByKeyword 基于关键词判断
func (o *orchestrator) shouldExtractByKeyword(userText, assistantText string) bool {
	// 检查消息长度
	minLen := o.config.Memory.MinMessageLength
	if len(userText) < minLen && len(assistantText) < minLen*2 {
		return false
	}

	// 检查记忆关键词
	memoryKeywords := []string{
		// 身份相关
		"我叫", "我是", "叫我", "称呼我", "我的名字", "我的职业", "我做", "我在",
		"以后叫我", "可以叫我", "你可以叫我", "你叫我",
		// 偏好相关
		"我喜欢", "我不喜欢", "我讨厌", "我偏好", "我习惯",
		"我常用", "我经常", "我一般", "我通常", "我倾向",
		// 目标相关
		"我想", "我要", "我计划", "我打算", "我的目标",
		"我希望", "我准备", "我会",
		// 技能/知识相关
		"我会", "我懂", "我了解", "我熟悉", "我擅长",
		"我学过", "我用过", "我做过", "我掌握",
		// 上下文相关
		"我用", "我的环境", "我的工具", "我的项目",
	}

	combined := userText + " " + assistantText
	for _, kw := range memoryKeywords {
		if strings.Contains(combined, kw) {
			if o.config.Base.Debug {
				log.Printf("触发记忆提取：检测到关键词 '%s'\n", kw)
			}
			return true
		}
	}

	return false
}

// shouldExtractByLLM 使用 LLM 分类器判断
func (o *orchestrator) shouldExtractByLLM(userText, assistantText string) bool {
	// 检查消息长度（太短直接跳过，节省 API 调用）
	if len(userText) < 5 && len(assistantText) < 10 {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
	defer cancel()

	// 构建分类提示词
	prompt := fmt.Sprintf(`判断以下对话是否包含值得长期保存的个人信息。

个人信息包括：
- 身份：姓名、昵称、职业、年龄、技能
- 偏好：喜好、风格、习惯
- 目标：学习目标、职业目标、项目计划
- 知识：专业技能、经验、专长
- 上下文：使用的工具、环境、约束

对话：
用户: %s
助手: %s

只回答 YES 或 NO。
回答：`, userText, assistantText)

	msgs := []models.ChatMessage{
		{Role: "user", Content: prompt},
	}

	// 使用独立的分类器客户端
	classifierModel := o.config.Classifier.Model
	response, err := o.classifierClient.Chat(ctx, msgs, classifierModel)
	if err != nil {
		if o.config.Base.Debug {
			log.Printf("LLM 分类器调用失败: %v，回退到保守策略\n", err)
		}
		// 失败时回退到保守策略
		return o.shouldExtractConservative(userText, assistantText)
	}

	response = strings.TrimSpace(strings.ToUpper(response))
	shouldExtract := strings.Contains(response, "YES")

	if o.config.Base.Debug {
		log.Printf("LLM 分类器判断: %s -> %v\n", response, shouldExtract)
	}

	return shouldExtract
}

// shouldExtractConservative 保守策略（信任提取器的判断）
func (o *orchestrator) shouldExtractConservative(userText, assistantText string) bool {
	minLen := o.config.Memory.MinMessageLength

	// 消息足够长就提取，让提取器自己判断是否有价值
	if len(userText) >= minLen || len(assistantText) >= minLen*2 {
		if o.config.Base.Debug {
			log.Printf("触发记忆提取：消息长度满足条件（保守策略）\n")
		}
		return true
	}

	return false
}
