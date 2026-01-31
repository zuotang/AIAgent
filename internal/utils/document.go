package utils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Document 表示一个文档，包含内容和元数据
type Document struct {
	Content  string            // 文档内容
	Metadata map[string]string // 元数据，如文件路径、行号等
}

// Chunk 表示文档的一个分块
type Chunk struct {
	Content  string            // 分块内容
	Metadata map[string]string // 元数据，如文件路径、起始行号、结束行号等
}

// ReadFile 读取文件内容并返回Document
func ReadFile(filePath string) (*Document, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// 创建元数据
	metadata := map[string]string{
		"source": filePath,
		"name":   filepath.Base(filePath),
	}

	return &Document{
		Content:  string(content),
		Metadata: metadata,
	}, nil
}

// ReadDirectory 读取目录中的所有支持的文件并返回Document列表
func ReadDirectory(dirPath string) ([]*Document, error) {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", dirPath)
	}

	var documents []*Document

	// 遍历目录
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 只处理文本文件
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".txt" || ext == ".md" || ext == ".go" || ext == ".py" || ext == ".js" || ext == ".json" || ext == ".yaml" || ext == ".yml" {
			doc, err := ReadFile(path)
			if err != nil {
				// 记录错误但继续处理其他文件
				fmt.Printf("Error reading file %s: %v\n", path, err)
				return nil
			}
			documents = append(documents, doc)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %v", err)
	}

	return documents, nil
}

// SplitDocument 将文档分割成多个分块
func SplitDocument(doc *Document, chunkSize, chunkOverlap int) []*Chunk {
	return SplitDocumentByTokens(doc, chunkSize, chunkOverlap)
}

// SplitDocumentByTokens 基于Token的文档分块
func SplitDocumentByTokens(doc *Document, maxTokens, tokenOverlap int) []*Chunk {
	var chunks []*Chunk
	content := doc.Content

	// 按行分割内容以获取行号信息
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// 合并行以创建分块
	currentChunk := ""
	currentStartLine := 1

	for i, line := range lines {
		// 尝试添加当前行
		lineWithNewline := line + "\n"
		potentialChunk := currentChunk + lineWithNewline

		// 计算token数
		potentialTokens := EstimateTokens(potentialChunk)

		// 如果添加后超过了maxTokens，且当前chunk不为空，则创建一个分块
		if potentialTokens > maxTokens && len(currentChunk) > 0 {
			// 创建分块
			chunk := &Chunk{
				Content: currentChunk,
				Metadata: map[string]string{
					"source":       doc.Metadata["source"],
					"name":         doc.Metadata["name"],
					"start_line":   fmt.Sprintf("%d", currentStartLine),
					"end_line":     fmt.Sprintf("%d", i),
					"token_count":  fmt.Sprintf("%d", EstimateTokens(currentChunk)),
					"chunk_type":   "token_based",
				},
			}
			chunks = append(chunks, chunk)

			// 计算下一个分块的起始位置，考虑重叠
			// 回退到重叠位置
			currentChunk = ""
			currentStartLine = i - tokenOverlap + 1
			if currentStartLine < 1 {
				currentStartLine = 1
			}

			// 重新添加重叠部分的行
			for j := currentStartLine - 1; j <= i; j++ {
				if j < totalLines {
					currentChunk += lines[j] + "\n"
				}
			}
		} else {
			// 继续添加到当前分块
			currentChunk = potentialChunk
		}
	}

	// 添加最后一个分块
	if len(currentChunk) > 0 {
		chunk := &Chunk{
			Content: currentChunk,
			Metadata: map[string]string{
				"source":       doc.Metadata["source"],
				"name":         doc.Metadata["name"],
				"start_line":   fmt.Sprintf("%d", currentStartLine),
				"end_line":     fmt.Sprintf("%d", totalLines),
				"token_count":  fmt.Sprintf("%d", EstimateTokens(currentChunk)),
				"chunk_type":   "token_based",
			},
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// SplitDocumentBySemantics 基于语义的文档分块
func SplitDocumentBySemantics(doc *Document, maxTokens, tokenOverlap int) []*Chunk {
	var chunks []*Chunk
	content := doc.Content

	// 按段落分割
	paragraphs := splitByParagraphs(content)

	currentChunk := ""
	currentParagraphIndex := 0

	for i, paragraph := range paragraphs {
		// 尝试添加当前段落
		potentialChunk := currentChunk
		if currentChunk != "" {
			potentialChunk += "\n\n"
		}
		potentialChunk += paragraph

		// 计算token数
		potentialTokens := EstimateTokens(potentialChunk)

		// 如果添加后超过了maxTokens，且当前chunk不为空，则创建一个分块
		if potentialTokens > maxTokens && len(currentChunk) > 0 {
			// 创建分块
			chunk := &Chunk{
				Content: currentChunk,
				Metadata: map[string]string{
					"source":         doc.Metadata["source"],
					"name":           doc.Metadata["name"],
					"start_paragraph": fmt.Sprintf("%d", currentParagraphIndex),
					"end_paragraph":   fmt.Sprintf("%d", i-1),
					"token_count":     fmt.Sprintf("%d", EstimateTokens(currentChunk)),
					"chunk_type":      "semantic",
				},
			}
			chunks = append(chunks, chunk)

			// 计算下一个分块的起始位置，考虑重叠
			currentParagraphIndex = i - tokenOverlap + 1
			if currentParagraphIndex < 0 {
				currentParagraphIndex = 0
			}

			// 重新添加重叠部分的段落
			currentChunk = ""
			for j := currentParagraphIndex; j <= i; j++ {
				if j < len(paragraphs) {
					if currentChunk != "" {
						currentChunk += "\n\n"
					}
					currentChunk += paragraphs[j]
				}
			}
		} else {
			// 继续添加到当前分块
			currentChunk = potentialChunk
		}
	}

	// 添加最后一个分块
	if len(currentChunk) > 0 {
		// 确保最后一个分块不会过大
		if EstimateTokens(currentChunk) > maxTokens*2 {
			// 如果过大，将其分割成更小的块
			tempChunks := SplitDocumentByTokens(&Document{
				Content:  currentChunk,
				Metadata: doc.Metadata,
			}, maxTokens, tokenOverlap)
			for _, chunk := range tempChunks {
				// 更新分块类型为semantic
				chunk.Metadata["chunk_type"] = "semantic"
				chunks = append(chunks, chunk)
			}
		} else {
			chunk := &Chunk{
				Content: currentChunk,
				Metadata: map[string]string{
					"source":         doc.Metadata["source"],
					"name":           doc.Metadata["name"],
					"start_paragraph": fmt.Sprintf("%d", currentParagraphIndex),
					"end_paragraph":   fmt.Sprintf("%d", len(paragraphs)-1),
					"token_count":     fmt.Sprintf("%d", EstimateTokens(currentChunk)),
					"chunk_type":      "semantic",
				},
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// splitByParagraphs 按段落分割文本
func splitByParagraphs(text string) []string {
	// 按多个换行符分割
	paragraphs := strings.Split(text, "\n\n")
	var result []string

	for _, p := range paragraphs {
		// 去除前后空白
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

// SplitText 分割文本为多个分块
func SplitText(text string, chunkSize, chunkOverlap int) []string {
	if text == "" {
		return []string{}
	}

	// 使用分词器分割文本
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return []string{}
	}

	// 分块处理
	var chunks []string
	for i := 0; i < len(tokens); i += chunkSize - chunkOverlap {
		end := i + chunkSize
		if end > len(tokens) {
			end = len(tokens)
		}

		// 构建分块内容
		content := joinTokens(tokens[i:end])
		chunks = append(chunks, content)
	}

	return chunks
}

// tokenize 将文本分割成标记
func tokenize(text string) []string {
	// 简单实现：按空格分割
	return strings.Fields(text)
}

// joinTokens 将标记重新组合成文本
func joinTokens(tokens []string) string {
	// 简单实现：用空格连接
	return strings.Join(tokens, " ")
}

// GetFileLines 获取文件的行数
func GetFileLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return lineCount, nil
}