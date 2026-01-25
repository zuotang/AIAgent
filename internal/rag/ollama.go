package rag

import (
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/ollama"
)

type Models struct {
	ChatLLM  *ollama.LLM
	Embedder embeddings.Embedder
}

func NewModels(chatModel, embedModel, ollamaURL string) (*Models, error) {
	chatLLM, err := ollama.New(
		ollama.WithModel(chatModel),
		ollama.WithServerURL(ollamaURL),
	)
	if err != nil {
		return nil, err
	}

	embedLLM, err := ollama.New(
		ollama.WithModel(embedModel),
		ollama.WithServerURL(ollamaURL),
	)
	if err != nil {
		return nil, err
	}

	embedder, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, err
	}

	return &Models{ChatLLM: chatLLM, Embedder: embedder}, nil
}
