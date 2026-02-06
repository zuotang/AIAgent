package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"agent-langchain/internal/models"
	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/engine"
	"agent-langchain/internal/workflow/nodes"
	"agent-langchain/internal/workflow/registry"
)

func main() {
	// 1. 创建注册中心并注册内置节点
	reg := registry.NewRegistry()
	if err := nodes.RegisterBuiltinNodes(reg); err != nil {
		log.Fatalf("Failed to register nodes: %v", err)
	}

	// 2. 加载工作流
	wf, err := dsl.Load("internal/workflow/example_workflow.json")
	if err != nil {
		log.Fatalf("Failed to load workflow: %v", err)
	}

	fmt.Printf("Loaded workflow: %s (ID: %s)\n", wf.Meta.Name, wf.Meta.ID)
	fmt.Printf("Nodes: %d, Edges: %d\n", len(wf.Nodes), len(wf.Edges))

	// 3. 创建执行器
	executor := engine.NewExecutor(reg)

	// 4. 创建运行时上下文（注入依赖）
	llmClient := models.New("http://localhost:11434", "qwen2.5:7b", "")

	rc := &registry.RunContext{
		LLMClient: registry.NewLLMClientAdapter(llmClient),
		// TODO: 注入其他依赖
		// EmbedClient:  embedClient,
		// MemoryStore:  memoryStore,
		// QdrantClient: qdrantClient,
		// ToolRegistry: toolRegistry,
	}

	// 5. 执行工作流
	fmt.Println("\nExecuting workflow...")
	trace, err := executor.Execute(context.Background(), wf, rc)
	if err != nil {
		log.Printf("Execution failed: %v\n", err)
	}

	// 6. 输出执行结果
	fmt.Printf("\n=== Execution Trace ===\n")
	fmt.Printf("Workflow: %s\n", trace.WorkflowName)
	fmt.Printf("Status: %s\n", trace.Status)
	fmt.Printf("Duration: %v\n", trace.Duration)

	if trace.Error != "" {
		fmt.Printf("Error: %s\n", trace.Error)
	}

	fmt.Printf("\n=== Node Traces ===\n")
	for nodeID, nodeTrace := range trace.Nodes {
		fmt.Printf("\nNode: %s (%s)\n", nodeID, nodeTrace.NodeType)
		fmt.Printf("  Status: %s\n", nodeTrace.Status)
		fmt.Printf("  Duration: %v\n", nodeTrace.Duration)

		if nodeTrace.Error != "" {
			fmt.Printf("  Error: %s\n", nodeTrace.Error)
		}

		if len(nodeTrace.Inputs) > 0 {
			fmt.Printf("  Inputs:\n")
			for port, value := range nodeTrace.Inputs {
				fmt.Printf("    %s: %v\n", port, truncate(fmt.Sprintf("%v", value), 100))
			}
		}

		if len(nodeTrace.Outputs) > 0 {
			fmt.Printf("  Outputs:\n")
			for port, value := range nodeTrace.Outputs {
				fmt.Printf("    %s: %v\n", port, truncate(fmt.Sprintf("%v", value), 100))
			}
		}
	}

	// 7. 输出 JSON 格式的 trace（可选）
	fmt.Printf("\n=== JSON Trace ===\n")
	traceJSON, _ := json.MarshalIndent(trace, "", "  ")
	fmt.Println(string(traceJSON))
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
