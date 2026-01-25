package rag

import (
	"net/url"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/vectorstores/qdrant"
)

func NewQdrantStore(
	qdrantURL string,
	collection string,
	embedder embeddings.Embedder,
) (qdrant.Store, error) {

	u, err := url.Parse(qdrantURL)
	if err != nil {
		return qdrant.Store{}, err
	}

	return qdrant.New(
		qdrant.WithURL(*u),
		qdrant.WithCollectionName(collection),
		qdrant.WithEmbedder(embedder),
	)
}
