package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-langchain/internal/agent"
	"agent-langchain/internal/config"
	"agent-langchain/internal/memory"
	"agent-langchain/internal/models"
	"agent-langchain/internal/rag"
	"agent-langchain/internal/utils"
)

// Orchestrator 编排器，负责协调各个组件
type Orchestrator struct {
	cfg          *config.Config
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
) *Orchestrator {
	return &Orchestrator{
		cfg:          cfg,
		llmClient:    llmClient,
		ollamaClient: ollamaClient,
		memStore:     memStore,
		vectorStore:  vectorStore,
		agent:        ag,
		chatModel:    chatModel,
	}
}

// ProcessMessage 处理用户消息
func (o *Orchestrator) ProcessMessage(
	ctx context.Context,
	userID string,
	userText string,
	conversationHistory string,
	systemPrompt string,
) (agent.Output, error) {
	// 1. 并行查询记忆
	structuredText, semanticDocs, err := o.retrieveMemories(ctx, userID, userText)
	if err != nil {
		return agent.Output{}, err
	}

	// 2. 显示上下文统计（debug模式）
	if o.cfg.Debug {
		o.showContextStats(systemPrompt, structuredText, semanticDocs, conversationHistory, userText)
	}

	// 3. 构建Agent输入
	input := agent.Input{
		UserID:       userID,
		Message:      userText,
		SystemPrompt: systemPrompt,
		Memory:       o.formatMemories(structuredText, semanticDocs),
		Conversation: conversationHistory,
	}

	// 4. 执行Agent
	output, err := o.agent.Run(ctx, input)
	if err != nil {
		return agent.Output{}, err
	}

	// 5. 提取并存储记忆
	if err := o.extractAndStoreMemories(ctx, userID, conversationHistory, userText, output.Response); err != nil {
		if o.cfg.Debug {
			fmt.Printf("提取记忆失败: %v\n", err)
		}
	}

	return output, nil
}

// retrieveMemories 并行查询结构化记忆和语义记忆
func (o *Orchestrator) retrieveMemories(
	ctx context.Context,
	userID string,
	query string,
) (string, []rag.Doc, error) {
	type result struct {
		structured string
		semantic   []rag.Doc
		err        error
	}

	structuredCh := make(chan result, 1)
	semanticCh := make(chan result, 1)

	// 并行查询 SQLite
	go func() {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.cfg.Timeout)*time.Second)
		defer cancel()
		text, err := o.memStore.RenderStructuredMemory(callCtx, userID, 30)
		structuredCh <- result{structured: text, err: err}
	}()

	// 并行查询 Qdrant
	go func() {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.cfg.Timeout)*time.Second)
		defer cancel()
		docs, err := o.vectorStore.SimilaritySearch(callCtx, userID, query, o.cfg.Qdrant.TopK)
		semanticCh <- result{semantic: docs, err: err}
	}()

	// 收集结果
	structuredResult := <-structuredCh
	semanticResult := <-semanticCh

	if structuredResult.err != nil && o.cfg.Debug {
		fmt.Printf("SQLite 查询失败: %v\n", structuredResult.err)
	}
	if semanticResult.err != nil {
		return "", nil, semanticResult.err
	}

	return structuredResult.structured, semanticResult.semantic, nil
}

// formatMemories 格式化记忆为文本
func (o *Orchestrator) formatMemories(structured string, semantic []rag.Doc) string {
	var sb strings.Builder

	sb.WriteString("【结构化长期记忆(SQLite)】\n")
	sb.WriteString(structured)
	sb.WriteString("\n\n")

	sb.WriteString("【语义长期记忆(Qdrant)】\n")
	for i, doc := range semantic {
		if i >= o.cfg.Qdrant.TopK {
			break
		}
		sb.WriteString("- " + truncate(doc.PageContent, 220) + "\n")
	}

	return sb.String()
}

// showContextStats 显示上下文统计
func (o *Orchestrator) showContextStats(
	systemPrompt string,
	structuredText string,
	semanticDocs []rag.Doc,
	conversation string,
	userInput string,
) {
	memory := structuredText + "\n"
	for i, doc := range semanticDocs {
		if i >= o.cfg.Qdrant.TopK {
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
	fmt.Print(utils.FormatContextStats(stats))
}

// extractAndStoreMemories 提取并存储记忆
func (o *Orchestrator) extractAndStoreMemories(
	ctx context.Context,
	userID string,
	conversationHistory string,
	userText string,
	assistantText string,
) error {
	// 使用记忆提取器模型
	extractorModel := o.cfg.Memory.ExtractorModel
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
		o.cfg.Debug,
		extractorModel,
	)
	if err != nil {
		return err
	}

	if len(memories) == 0 {
		return nil
	}

	// 存储到 SQLite
	if err := o.storeStructuredMemories(ctx, userID, memories); err != nil {
		return err
	}

	// 存储到 Qdrant
	if err := o.storeSemanticMemories(ctx, userID, memories); err != nil {
		return err
	}

	return nil
}

// storeStructuredMemories 存储结构化记忆
func (o *Orchestrator) storeStructuredMemories(
	ctx context.Context,
	userID string,
	memories []memory.ExtractedMemory,
) error {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.cfg.Timeout)*time.Second)
	defer cancel()

	if err := o.memStore.UpsertExtractedMemories(callCtx, userID, memories); err != nil {
		if o.cfg.Debug {
			fmt.Printf("写入 SQLite 失败: %v\n", err)
		}
		return err
	}

	if o.cfg.Debug {
		fmt.Printf("成功写入 SQLite 结构化记忆\n")
	}
	return nil
}

// storeSemanticMemories 存储语义记忆
func (o *Orchestrator) storeSemanticMemories(
	ctx context.Context,
	userID string,
	memories []memory.ExtractedMemory,
) error {
	var vectorTexts []string
	seen := make(map[string]struct{})

	for _, m := range memories {
		if !m.AlsoVector || m.Confidence < 0.65 {
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

	if o.cfg.Debug {
		fmt.Printf("准备写入 Qdrant 的语义记忆数量: %d\n", len(vectorTexts))
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.cfg.Timeout)*time.Second)
	defer cancel()

	if err := o.vectorStore.UpsertTexts(callCtx, userID, vectorTexts); err != nil {
		if o.cfg.Debug {
			fmt.Printf("写入 Qdrant 失败: %v\n", err)
		}
		return err
	}

	if o.cfg.Debug {
		fmt.Printf("成功写入 Qdrant 语义记忆\n")
	}
	return nil
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
