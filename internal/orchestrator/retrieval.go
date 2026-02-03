package orchestrator

import (
	"context"
	"log"
	"time"

	"agent-langchain/internal/rag"
)

// ContextResult 上下文检索结果
type ContextResult struct {
	MemoryText     string
	KnowledgeText  string
	StructuredText string
	SemanticDocs   []rag.Doc
	KnowledgeDocs  []rag.Doc
}

// retrieveAndMergeContext 根据查询类型检索并合并上下文
// 这个函数提取了 ProcessMessage 和 ProcessMessageStream 中的重复逻辑
func (o *orchestrator) retrieveAndMergeContext(
	ctx context.Context,
	userID string,
	agentID uint,
	userText string,
	queryType string,
) (*ContextResult, error) {
	result := &ContextResult{}

	// 根据分类结果，条件性地检索上下文
	switch queryType {
	case "BOTH":
		// 并行检索记忆和知识库
		result = o.retrieveBoth(ctx, userID, agentID, userText)

	case "MEMORY":
		// 只检索记忆
		result = o.retrieveMemoryOnly(ctx, userID, agentID, userText)

	case "KNOWLEDGE":
		// 只检索知识库
		result = o.retrieveKnowledgeOnly(ctx, userID, agentID, userText)

	case "NONE":
		// 不检索任何内容
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 查询类型为 NONE，跳过上下文检索")
		}
	}

	return result, nil
}

// retrieveBoth 并行检索记忆和知识库
func (o *orchestrator) retrieveBoth(
	ctx context.Context,
	userID string,
	agentID uint,
	userText string,
) *ContextResult {
	type result struct {
		structured string
		semantic   []rag.Doc
		knowledge  []rag.Doc
		err        error
	}

	memoryCh := make(chan result, 1)
	knowledgeCh := make(chan result, 1)

	// 并行检索记忆
	go func() {
		structured, semantic, err := o.retrieveMemories(ctx, userID, agentID, userText)
		memoryCh <- result{structured: structured, semantic: semantic, err: err}
	}()

	// 并行检索知识库
	go func() {
		knowledge, err := o.retrieveKnowledge(ctx, userID, agentID, userText)
		knowledgeCh <- result{knowledge: knowledge, err: err}
	}()

	// 收集结果
	memoryResult := <-memoryCh
	knowledgeResult := <-knowledgeCh

	contextResult := &ContextResult{}

	if memoryResult.err == nil {
		contextResult.StructuredText = memoryResult.structured
		contextResult.SemanticDocs = memoryResult.semantic
		contextResult.MemoryText = o.formatMemories(memoryResult.structured, memoryResult.semantic)
	} else if o.config.Base.Debug {
		log.Printf("[DEBUG] 记忆检索失败: %v", memoryResult.err)
	}

	if knowledgeResult.err == nil {
		contextResult.KnowledgeDocs = knowledgeResult.knowledge
		contextResult.KnowledgeText = o.formatKnowledge(knowledgeResult.knowledge)
	} else if o.config.Base.Debug {
		log.Printf("[DEBUG] 知识库检索失败: %v", knowledgeResult.err)
	}

	return contextResult
}

// retrieveMemoryOnly 只检索记忆
func (o *orchestrator) retrieveMemoryOnly(
	ctx context.Context,
	userID string,
	agentID uint,
	userText string,
) *ContextResult {
	contextResult := &ContextResult{}

	structured, semantic, err := o.retrieveMemories(ctx, userID, agentID, userText)
	if err != nil {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 记忆检索失败: %v", err)
		}
	} else {
		contextResult.StructuredText = structured
		contextResult.SemanticDocs = semantic
		contextResult.MemoryText = o.formatMemories(structured, semantic)
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 记忆已加载")
		}
	}

	return contextResult
}

// retrieveKnowledgeOnly 只检索知识库
func (o *orchestrator) retrieveKnowledgeOnly(
	ctx context.Context,
	userID string,
	agentID uint,
	userText string,
) *ContextResult {
	contextResult := &ContextResult{}

	knowledge, err := o.retrieveKnowledge(ctx, userID, agentID, userText)
	if err != nil {
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 知识库检索失败: %v", err)
		}
	} else {
		contextResult.KnowledgeDocs = knowledge
		contextResult.KnowledgeText = o.formatKnowledge(knowledge)
		if o.config.Base.Debug {
			log.Printf("[DEBUG] 知识库已加载")
		}
	}

	return contextResult
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

	// 并行查询 Qdrant（语义记忆）
	go func() {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Base.Timeout)*time.Second)
		defer cancel()
		docs, err := o.vectorStore.SimilaritySearchMemory(callCtx, userID, agentID, query, o.config.Storage.Qdrant.TopK)
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

	// 打印记忆检索结果
	if o.config.Base.Debug {
		log.Printf("[DEBUG] ========== 记忆检索结果 ==========")
		log.Printf("[DEBUG] 查询内容: %s", query)

		// 打印结构化记忆
		log.Printf("[DEBUG] 结构化记忆:")
		if structuredText != "" && structuredText != "(暂无长期记忆)" {
			log.Printf("[DEBUG] %s", structuredText)
		} else {
			log.Printf("[DEBUG]   (暂无结构化记忆)")
		}

		// 打印语义记忆（过滤前）
		log.Printf("[DEBUG] 语义记忆 (检索到 %d 条，最小相似度阈值: %.2f):", len(semanticResult.semantic), o.config.Storage.Qdrant.MinScore)
	}

	// 过滤低相似度的语义记忆
	filteredSemantic := make([]rag.Doc, 0, len(semanticResult.semantic))
	for _, doc := range semanticResult.semantic {
		if doc.Score >= o.config.Storage.Qdrant.MinScore {
			filteredSemantic = append(filteredSemantic, doc)
		} else if o.config.Base.Debug {
			log.Printf("[DEBUG] 过滤低相似度记忆: %.4f < %.2f - %s", doc.Score, o.config.Storage.Qdrant.MinScore, doc.PageContent)
		}
	}

	// 打印过滤后的语义记忆
	if o.config.Base.Debug {
		log.Printf("[DEBUG] 过滤后保留 %d 条语义记忆:", len(filteredSemantic))
		for i, doc := range filteredSemantic {
			log.Printf("[DEBUG] 语义记忆 %d:", i+1)
			log.Printf("[DEBUG]   内容: %s", doc.PageContent)
			log.Printf("[DEBUG]   相似度分数: %.4f", doc.Score)
			log.Printf("[DEBUG]   ---")
		}
		log.Printf("[DEBUG] ===================================")
	}

	return structuredText, filteredSemantic, nil
}

// retrieveKnowledge 检索知识库
func (o *orchestrator) retrieveKnowledge(
	ctx context.Context,
	userID string,
	agentID uint,
	query string,
) ([]rag.Doc, error) {
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(o.config.Base.Timeout)*time.Second)
	defer cancel()

	// 从 Qdrant 检索知识库内容
	// 知识库是共享的，只需要 agent_id，不需要 user_id
	topK := o.config.Knowledge.TopK
	if topK == 0 {
		topK = 3
	}

	docs, err := o.vectorStore.SimilaritySearchKnowledge(callCtx, agentID, query, topK)
	if err != nil {
		return nil, err
	}

	// 打印知识库检索结果（过滤前）
	if o.config.Base.Debug {
		log.Printf("[DEBUG] ========== 知识库检索结果 ==========")
		log.Printf("[DEBUG] 查询内容: %s", query)
		log.Printf("[DEBUG] 检索到 %d 条知识库文档（最小相似度阈值: %.2f）:", len(docs), o.config.Knowledge.MinScore)
	}

	// 过滤低相似度的知识库文档
	filteredDocs := make([]rag.Doc, 0, len(docs))
	for _, doc := range docs {
		if doc.Score >= o.config.Knowledge.MinScore {
			filteredDocs = append(filteredDocs, doc)
		} else if o.config.Base.Debug {
			log.Printf("[DEBUG] 过滤低相似度文档: %.4f < %.2f - %s", doc.Score, o.config.Knowledge.MinScore, doc.PageContent)
		}
	}

	// 打印过滤后的知识库文档
	if o.config.Base.Debug {
		log.Printf("[DEBUG] 过滤后保留 %d 条知识库文档:", len(filteredDocs))
		for i, doc := range filteredDocs {
			log.Printf("[DEBUG] 文档 %d:", i+1)
			log.Printf("[DEBUG]   内容: %s", doc.PageContent)
			log.Printf("[DEBUG]   相似度分数: %.4f", doc.Score)
			log.Printf("[DEBUG]   ---")
		}
		log.Printf("[DEBUG] ===================================")
	}

	return filteredDocs, nil
}
