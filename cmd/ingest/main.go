package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"agent-langchain/internal/models"
	"agent-langchain/internal/rag"
)

func main() {
	// 解析命令行参数
	var (
	filePath     = flag.String("file", "", "单个文件路径")
	dirPath      = flag.String("dir", "", "目录路径（批量处理）")
	qdrantURL    = flag.String("qdrant", "http://127.0.0.1:6333", "Qdrant服务器URL")
	collection   = flag.String("collection", "memories", "Qdrant集合名称（用于存储所有数据）")
	embedModel   = flag.String("embed", "nomic-embed-text", "嵌入模型名称")
	userID       = flag.String("user", "default", "用户ID")
	chunkSize    = flag.Int("chunk-size", 1000, "文本分块大小")
	chunkOverlap = flag.Int("chunk-overlap", 100, "分块重叠大小")
	chunkingStrategy = flag.String("strategy", "tokens", "分块策略：tokens（基于Token）或 semantic（基于语义）")
)
	flag.Parse()

	// 检查参数
	if *filePath == "" && *dirPath == "" {
		fmt.Println("错误：必须指定 --file 或 --dir 参数")
		flag.Usage()
		os.Exit(1)
	}

	if *filePath != "" && *dirPath != "" {
		fmt.Println("错误：只能指定 --file 或 --dir 参数中的一个")
		flag.Usage()
		os.Exit(1)
	}

	// 初始化 Ollama 客户端
	ollama := models.New("http://127.0.0.1:11434", "gemma3:4b", *embedModel)
	if ollama == nil {
		log.Fatalf("初始化 Ollama 客户端失败")
	}

	// 创建 Qdrant 存储
	store := rag.NewStoreFromOllama(*qdrantURL, "", *collection, ollama)

	// 创建 Ingestor
	strategy := rag.ChunkingStrategyTokens
	if *chunkingStrategy == "semantic" {
		strategy = rag.ChunkingStrategySemantic
	}

	ingestor := rag.NewIngestor(store, *chunkSize, *chunkOverlap, strategy, *userID)

	// 创建上下文
	ctx := context.Background()

	// 执行录入
	if *filePath != "" {
		fmt.Printf("正在录入文件: %s\n", *filePath)
		if err := ingestor.IngestFile(ctx, *filePath); err != nil {
			log.Fatalf("录入文件失败: %v", err)
		}
		fmt.Printf("文件录入成功: %s\n", *filePath)
	} else if *dirPath != "" {
		fmt.Printf("正在录入目录: %s\n", *dirPath)
		if err := ingestor.IngestDirectory(ctx, *dirPath); err != nil {
			log.Fatalf("录入目录失败: %v", err)
		}
		fmt.Printf("目录录入完成: %s\n", *dirPath)
	}
}
