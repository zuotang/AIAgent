package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"agent-langchian/internal/rag"

	"github.com/tmc/langchaingo/schema"
)

func main() {
	var (
		ollamaURL  = flag.String("ollama", "http://127.0.0.1:11434", "Ollama server URL")
		qdrantURL  = flag.String("qdrant", "http://127.0.0.1:6333", "Qdrant URL")
		collection = flag.String("col", "my_collection", "Qdrant collection")
		chatModel  = flag.String("chat", "gemma3:4b", "Chat model")
		embedModel = flag.String("embed", "nomic-embed-text", "Embedding model")
		filePath   = flag.String("file", "data/fortune.txt", "Text file to ingest")
		src        = flag.String("src", "fortune", "Metadata source tag")
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

	f, err := os.Open(*filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var docs []schema.Document
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		docs = append(docs, schema.Document{
			PageContent: line,
			Metadata:    map[string]any{"src": *src},
		})
	}
	if err := sc.Err(); err != nil {
		log.Fatal(err)
	}

	if len(docs) == 0 {
		log.Println("no docs found to ingest")
		return
	}

	_, err = store.AddDocuments(ctx, docs)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("ingested %d docs into %s\n", len(docs), *collection)
}
