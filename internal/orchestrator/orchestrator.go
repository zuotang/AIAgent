package orchestrator

import (
	"context"
	"fmt"
	"log"
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
	ProcessMessage(ctx context.Context, userID string, agentID uint, userText string, conversationHistory string, systemPrompt string) (agent.Output, error)
	ProcessMessageStream(ctx context.Context, userID string, agentID uint, userText string, conversationHistory string, systemPrompt string, callback func(string) error) (agent.Output, error)
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
	config       *config.Config
	llmClient    models.LLMClient
	ollamaClient *models.Client
	memStore     *memory.Store
	vectorStore  *rag.QdrantStore
	agent        agent.Agent
	chatModel    string
}

// New 创建新的编排器
func New(
	cfg *config.Config,
	llmClient models.LLMClient,
	ollamaClient *models.Client,
	memStore *memory.Store,
	vectorStore *rag.QdrantStore,
	ag agent.Agent,
	chatModel string,
) Orchestrator {
	return &orchestrator{
		config:       cfg,
		llmClient:    llmClient,
		ollamaClient: ollamaClient,
		memStore:     memStore,
		vectorStore:  vectorStore,
		agent:        ag,
		chatModel:    chatModel,
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
	systemPrompt string,
) (agent.Output, error) {
	// 1. 按需加载记忆（剧情触发时）
	structuredText := ""
	var semanticDocs []rag.Doc
	memoryText := ""
	if o.shouldLoadMemory(userText) {
		var err error
		structuredText, semanticDocs, err = o.retrieveMemories(ctx, userID, agentID, userText)
		if err != nil {
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 按需记忆检索失败: %v", err)
			}
		} else {
			memoryText = o.formatMemories(structuredText, semanticDocs)
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 按需记忆已加载")
			}
		}
	}

	// 2. 显示上下文统计（debug模式）
	// if o.config.Base.Debug {
	// 	o.showContextStats(systemPrompt, structuredText, semanticDocs, conversationHistory, userText)
	// }

	// 3. 增量压缩对话上下文（基于 token 占用率触发）
	compressedContext := conversationHistory
	stats := utils.CalculateContextStats(systemPrompt, memoryText, conversationHistory, userText, o.chatModel)
	shouldCompress := ShouldCompressContextByTokens(stats, 60.0) // 保留约 40% 余量
	if o.config.Base.Debug {
		log.Printf("[DEBUG] 上下文占用率: %.1f%% (Total=%d, Limit=%d)", stats.UsagePercent, stats.TotalTokens, stats.ModelLimit)
	}

	if shouldCompress {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 触发增量压缩 - 当前上下文长度: %d", len(conversationHistory))
		}

		// 使用 extractor 模型进行增量压缩
		compressorModel := o.config.Extractor.Model
		if compressorModel == "" {
			compressorModel = o.chatModel
		}

		// 注意：这里传入0作为lastMessageID，因为压缩发生在保存新消息之前
		// 实际的LastMessageID会在保存消息后更新
		compressed, err := CompressContextIncremental(ctx, o.llmClient, o.memStore, userID, agentID, conversationHistory, 0, compressorModel, 200)
		if err != nil {
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 增量压缩失败: %v，使用原始上下文", err)
			}
			// 压缩失败，使用原始上下文
		} else {
			compressedContext = compressed
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 增量压缩完成 - 压缩后长度: %d", len(compressedContext))
				log.Printf("[DEBUG] 压缩后内容: %s", compressedContext)
			}
		}
	} else {
		// 未达到压缩阈值，使用上次压缩的上下文 + 加载的历史作为Agent输入
		lastCompressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID)
		if err == nil && lastCompressed.CompressedText != "" {
			// 有上次的压缩结果，追加加载的历史作为上下文
			compressedContext = lastCompressed.CompressedText + "\n\n" + conversationHistory
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 使用上次压缩 + 加载历史作为上下文 - 总长度: %d", len(compressedContext))
			}
		} else {
			// 没有上次的压缩结果，直接使用原始上下文
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 首次对话或无压缩历史 - 使用原始上下文，长度: %d", len(conversationHistory))
			}
		}
	}

	// 4. 构建Agent输入（不使用记忆）
	input := agent.Input{
		UserID:       userID,
		Message:      userText,
		SystemPrompt: systemPrompt,
		Memory:       memoryText, // 按需记忆（仅本轮注入）
		Conversation: compressedContext,
	}

	// 4. 执行Agent
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
	assistantMsgID, err := o.memStore.SaveChatMessage(ctx, userID, "assistant", output.Response, sessionID, agentID)
	if err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 保存助手响应失败: %v", err)
	}

	// 更新压缩上下文的LastMessageID
	if assistantMsgID > 0 {
		lastCompressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID)
		if err == nil {
			// 如果未触发压缩，需要追加本次对话到压缩上下文
			if !shouldCompress {
				// 构建本次对话内容
				cleanUser := utils.PreprocessLite(userText)
				cleanAssistant := utils.PreprocessLite(output.Response)
				currentTurn := fmt.Sprintf("User: %s\nAssistant: %s", cleanUser, cleanAssistant)
				// 追加到压缩上下文
				lastCompressed.CompressedText = lastCompressed.CompressedText + "\n\n" + currentTurn
				if o.config.Base.Debug {
					log.Printf("[DEBUG] 追加本次对话到压缩上下文 - 新增长度: %d", len(currentTurn))
				}
			}
			// 更新LastMessageID为最新的助手消息ID
			lastCompressed.LastMessageID = assistantMsgID
			if err := o.memStore.UpsertCompressedContext(ctx, lastCompressed); err != nil && o.config.Base.Debug {
				log.Printf("[DEBUG] 更新LastMessageID失败: %v", err)
			} else if o.config.Base.Debug {
				log.Printf("[DEBUG] 已更新LastMessageID: %d", assistantMsgID)
			}
		} else if shouldCompress {
			// 如果没有压缩上下文但触发了压缩，创建一个新的
			newCompressed := &memory.CompressedContext{
				UserID:         userID,
				AgentID:        agentID,
				CompressedText: compressedContext,
				LastMessageID:  assistantMsgID,
			}
			if err := o.memStore.UpsertCompressedContext(ctx, newCompressed); err != nil && o.config.Base.Debug {
				log.Printf("[DEBUG] 创建压缩上下文失败: %v", err)
			}
		}
	}

	// 5. 异步提取并存储记忆（智能触发）
	if o.config.Memory.EnableExtractor {
		if o.shouldExtractMemory(userText, output.Response) {
			go func() {
				// 使用 background context 避免父 context 取消影响异步任务
				// 设置超时防止 goroutine 泄漏
				bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
				defer cancel()

				if err := o.extractAndStoreMemories(bgCtx, userID, agentID, conversationHistory, userText, output.Response); err != nil {
					if o.config.Base.Debug {
						log.Printf("[DEBUG] 异步提取记忆失败: %v", err)
					}
				}
			}()
		} else if o.config.Base.Debug {
			log.Printf("[DEBUG] 跳过记忆提取：未触发条件")
		}
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
	systemPrompt string,
	callback func(string) error,
) (agent.Output, error) {
	// 1. 按需加载记忆（剧情触发时）
	structuredText := ""
	var semanticDocs []rag.Doc
	memoryText := ""
	if o.shouldLoadMemory(userText) {
		var err error
		structuredText, semanticDocs, err = o.retrieveMemories(ctx, userID, agentID, userText)
		if err != nil {
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 按需记忆检索失败: %v", err)
			}
		} else {
			memoryText = o.formatMemories(structuredText, semanticDocs)
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 按需记忆已加载")
			}
		}
	}

	// 2. 显示上下文统计（debug模式）
	// if o.config.Base.Debug {
	// 	o.showContextStats(systemPrompt, structuredText, semanticDocs, conversationHistory, userText)
	// }

	// 3. 增量压缩对话上下文（基于 token 占用率触发）
	compressedContext := conversationHistory
	stats := utils.CalculateContextStats(systemPrompt, memoryText, conversationHistory, userText, o.chatModel)
	shouldCompressStream := ShouldCompressContextByTokens(stats, 60.0) // 保留约 40% 余量
	if o.config.Base.Debug {
		log.Printf("[DEBUG] 流式处理 - 上下文占用率: %.1f%% (Total=%d, Limit=%d)", stats.UsagePercent, stats.TotalTokens, stats.ModelLimit)
	}

	if shouldCompressStream {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 流式处理 - 触发增量压缩 - 当前上下文长度: %d", len(conversationHistory))
		}

		compressorModel := o.config.Extractor.Model
		if compressorModel == "" {
			compressorModel = o.chatModel
		}

		// 注意：这里传入0作为lastMessageID，因为压缩发生在保存新消息之前
		// 实际的LastMessageID会在保存消息后更新
		compressed, err := CompressContextIncremental(ctx, o.llmClient, o.memStore, userID, agentID, conversationHistory, 0, compressorModel, 200)
		if err != nil {
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 增量压缩失败: %v，使用原始上下文", err)
			}
		} else {
			compressedContext = compressed
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 流式处理 - 增量压缩完成 - 压缩后长度: %d", len(compressedContext))
			}
		}
	} else {
		// 未达到压缩阈值，使用上次压缩的上下文 + 加载的历史作为Agent输入
		lastCompressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID)
		if err == nil && lastCompressed.CompressedText != "" {
			// 有上次的压缩结果，追加加载的历史作为上下文
			compressedContext = lastCompressed.CompressedText + "\n\n" + conversationHistory
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 流式处理 - 使用上次压缩 + 加载历史作为上下文 - 总长度: %d", len(compressedContext))
			}
		} else {
			// 没有上次的压缩结果，直接使用原始上下文
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 流式处理 - 首次对话或无压缩历史 - 使用原始上下文，长度: %d", len(conversationHistory))
			}
		}
	}

	// 4. 构建Agent输入（不使用记忆）
	input := agent.Input{
		UserID:       userID,
		Message:      userText,
		SystemPrompt: systemPrompt,
		Memory:       memoryText, // 按需记忆（仅本轮注入）
		Conversation: compressedContext,
	}

	// 4. 执行Agent流式处理
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
	assistantMsgID, err := o.memStore.SaveChatMessage(ctx, userID, "assistant", output.Response, sessionID, agentID)
	if err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 保存助手响应失败: %v", err)
	}

	// 更新压缩上下文的LastMessageID
	if assistantMsgID > 0 {
		lastCompressed, err := o.memStore.GetCompressedContext(ctx, userID, agentID)
		if err == nil {
			// 更新LastMessageID为最新的助手消息ID
			lastCompressed.LastMessageID = assistantMsgID
			if err := o.memStore.UpsertCompressedContext(ctx, lastCompressed); err != nil && o.config.Base.Debug {
				log.Printf("[DEBUG] 更新LastMessageID失败: %v", err)
			} else if o.config.Base.Debug {
				log.Printf("[DEBUG] 流式处理 - 已更新LastMessageID: %d", assistantMsgID)
			}
		} else if ShouldCompressContext(conversationHistory, 4000) {
			// 如果没有压缩上下文但触发了压缩，创建一个新的
			newCompressed := &memory.CompressedContext{
				UserID:         userID,
				CompressedText: compressedContext,
				LastMessageID:  assistantMsgID,
			}
			if err := o.memStore.UpsertCompressedContext(ctx, newCompressed); err != nil && o.config.Base.Debug {
				log.Printf("[DEBUG] 创建压缩上下文失败: %v", err)
			}
		}
	}

	// 6. 异步提取并存储记忆（智能触发）
	if o.config.Memory.EnableExtractor {
		if o.shouldExtractMemory(userText, output.Response) {
			go func() {
				// 使用 background context 避免父 context 取消影响异步任务
				// 设置超时防止 goroutine 泄漏
				bgCtx, cancel := context.WithTimeout(context.Background(), time.Duration(o.config.Base.Timeout)*time.Second)
				defer cancel()

				if err := o.extractAndStoreMemories(bgCtx, userID, agentID, conversationHistory, userText, output.Response); err != nil {
					if o.config.Base.Debug {
						log.Printf("[DEBUG] 异步提取记忆失败: %v", err)
					}
				}
			}()
		} else if o.config.Base.Debug {
			log.Printf("[DEBUG] 跳过记忆提取：未触发条件")
		}
	} else if o.config.Base.Debug {
		log.Printf("[DEBUG] 记忆提取功能已禁用 (memory.enable_extractor=false)")
	}

	return output, nil
}

// retrieveMemories 并行查询结构化记忆和语义记忆
func (o *orchestrator) retrieveMemories(
	ctx context.Context,
	userID string,
	agentID uint,
	query string,
) (string, []rag.Doc, error) {
	type result struct {
		structured string
		semantic   []rag.Doc
		err        error
	}

	structuredCh := make(chan result, 1)
	semanticCh := make(chan result, 1)

	// 并行查询 SQLite（结构化记忆）
	go func() {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Base.Timeout)*time.Second)
		defer cancel()
		text, err := o.memStore.RenderStructuredMemory(callCtx, userID, agentID, 20)
		structuredCh <- result{structured: text, err: err}
	}()

	// 并行查询 Qdrant
	go func() {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Base.Timeout)*time.Second)
		defer cancel()
		docs, err := o.vectorStore.SimilaritySearch(callCtx, userID, agentID, query, o.config.Storage.Qdrant.TopK)
		semanticCh <- result{semantic: docs, err: err}
	}()

	// 收集结果
	structuredResult := <-structuredCh
	semanticResult := <-semanticCh

	structuredText := structuredResult.structured
	if structuredResult.err != nil && o.config.Base.Debug {
		log.Printf("[DEBUG] 结构化记忆查询失败: %v", structuredResult.err)
	}

	if semanticResult.err != nil {
		return "", nil, semanticResult.err
	}

	return structuredText, semanticResult.semantic, nil
}

// formatMemories 格式化记忆为文本
func (o *orchestrator) formatMemories(structured string, semantic []rag.Doc) string {
	var sb strings.Builder

	sb.WriteString("【角色记忆库（按需加载）】\n")
	if structured != "" && structured != "(暂无长期记忆)" {
		sb.WriteString("【结构化记忆】\n")
		sb.WriteString(structured)
		sb.WriteString("\n")
	}
	if len(semantic) > 0 {
		sb.WriteString("【剧情/互动线索】\n")
	}
	for i, doc := range semantic {
		if i >= o.config.Storage.Qdrant.TopK {
			break
		}
		sb.WriteString("- " + truncate(doc.PageContent, 220) + "\n")
	}

	return sb.String()
}

// showContextStats 显示上下文统计
func (o *orchestrator) showContextStats(
	systemPrompt string,
	structuredText string,
	semanticDocs []rag.Doc,
	conversation string,
	userInput string,
) {
	memory := structuredText + "\n"
	for i, doc := range semanticDocs {
		if i >= o.config.Storage.Qdrant.TopK {
			break
		}
		memory += doc.PageContent + "\n"
	}

	stats := utils.CalculateContextStats(
		systemPrompt,
		memory,
		conversation,
		userInput,
		o.chatModel,
	)
	log.Print(utils.FormatContextStats(stats))
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
	if extractorModel == "" {
		extractorModel = o.chatModel
	}

	// 提取记忆
	memories, err := ExtractMemories(
		ctx,
		o.llmClient,
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
		return nil
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

	if err := o.vectorStore.UpsertTexts(callCtx, userID, agentID, vectorTexts, ""); err != nil {
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
func (o *orchestrator) shouldLoadMemory(userText string) bool {
	if len(strings.TrimSpace(userText)) < o.config.Memory.OnDemandMinLength {
		return false
	}

	for _, kw := range o.config.Memory.OnDemandKeywords {
		if strings.Contains(userText, kw) {
			if o.config.Base.Debug {
				log.Printf("[DEBUG] 触发按需记忆加载: %s", kw)
			}
			return true
		}
	}

	return false
}

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

	// 使用配置的分类器模型
	classifierModel := o.config.Classifier.Model
	response, err := o.llmClient.Chat(ctx, msgs, classifierModel)
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

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func fingerprint(owner, typ, key, val string) string {
	// Simple fingerprint using concatenation
	return fmt.Sprintf("%s|%s|%s|%s", owner, typ, key, val)
}
