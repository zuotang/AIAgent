package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"agent-langchian/internal/memory"
	"agent-langchian/internal/profile"
	"agent-langchian/internal/rag"

	"github.com/tmc/langchaingo/llms"
)

// -------------------- main --------------------

func main() {
	var (
		ollamaURL  = flag.String("ollama", "http://127.0.0.1:11434", "Ollama server URL")
		qdrantURL  = flag.String("qdrant", "http://127.0.0.1:6333", "Qdrant URL")
		collection = flag.String("col", "my_collection", "Qdrant collection")
		chatModel  = flag.String("chat", "gemma3:4b", "Chat model")
		embedModel = flag.String("embed", "nomic-embed-text", "Embedding model")
		topK       = flag.Int("k", 6, "TopK retrieval")
		dbPath     = flag.String("db", "memory.db", "SQLite db path")
	)
	flag.Parse()

	ctx := context.Background()

	models, err := rag.NewModels(*chatModel, *embedModel, *ollamaURL)
	if err != nil {
		log.Fatal(err)
	}
	store, err := rag.NewQdrantStore(*qdrantURL, *collection, models.Embedder)
	if err != nil {
		log.Fatal(err)
	}

	reader := bufio.NewReader(os.Stdin)

	// 第一行作为 uid
	fmt.Print("请输入你的 UID（第一行输入将作为用户ID）：")
	first, _ := reader.ReadString('\n')
	first = strings.TrimSpace(first)
	uid := first
	if uid == "" {
		uid = "local"
	}

	mem, err := memory.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer mem.Close()

	userProfile, err := mem.LoadProfile(ctx, uid)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("算命智能体（用户=%s，输入 exit 退出）\n", uid)

	for {
		fmt.Print("\n你：")
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if text == "exit" {
			return
		}

		// 处理长期记忆 - 只有以"请记住"或"记住"开头的文本才会被处理
		if profile.IsMemoryCommand(text) {
			cleaned := strings.TrimSpace(strings.TrimPrefix(text, "请记住"))
			cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "记住"))

			if updated, notes := profile.ExtractAndUpdateProfile(cleaned, userProfile); updated {
				if err := mem.SaveProfile(ctx, uid, userProfile); err != nil {
					log.Fatal(err)
				}
				if len(notes) > 0 {
					fmt.Println("\n[记忆] " + strings.Join(notes, " "))
				}
			}
		}

		// ② 注入长期记忆
		profileJSON, _ := json.Marshal(userProfile)

		system := `
你是一位“娱乐向算命/占卜智能体”，风格沉稳但不吓人。
规则：仅供娱乐与自我反思；不做医疗/法律/金融结论；不给出具体灾祸日期/彩票号码；信息不足最多追问2个问题。
输出结构：总体、事业/学业、感情、财运、建议(3条)、娱乐声明。

【重要】用户长期记忆(JSON)里：
- agent 表示“你(智能体)”的设定
- user 表示“用户本人”的信息
严禁混用。
`
		system = system + "\n【用户长期记忆(JSON)】\n" + string(profileJSON) + "\n"

		// ③ RAG 检索（知识库）
		query := text + " 命理 解读 建议 话术 模板"
		docs, err := store.SimilaritySearch(ctx, query, *topK)
		if err != nil {
			log.Fatal(err)
		}

		var ctxText []string
		for _, d := range docs {
			ctxText = append(ctxText, d.PageContent)
		}

		prompt := fmt.Sprintf(
			`根据【资料摘录】对用户进行娱乐向解读。若资料不足，可给泛化建议，但要说明是泛化建议，不要编造具体事件。
你也要参考【用户长期记忆】来保持设定一致。

【资料摘录】
%s

【用户问题】
%s
`, strings.Join(ctxText, "\n---\n"), text)

		msgs := []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeSystem, system),
			llms.TextParts(llms.ChatMessageTypeHuman, prompt),
		}

		resp, err := models.ChatLLM.GenerateContent(ctx, msgs)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("\nAI：", resp.Choices[0].Content)
	}
}
