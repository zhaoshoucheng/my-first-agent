package genai

import (
	"context"

	"google.golang.org/genai"
)

// CreateEmbedding creates embeddings from texts.
func (g *Vertex) CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	contents := make([]*genai.Content, 0, len(texts))
	for _, t := range texts {
		contents = append(contents, &genai.Content{
			Parts: []*genai.Part{{Text: t}},
		})
	}
	resp, err := g.client.Models.EmbedContent(ctx, g.opts.DefaultEmbeddingModel, contents, nil)
	if err != nil {
		return nil, err
	}
	result := make([][]float32, 0, len(resp.Embeddings))
	for _, embedding := range resp.Embeddings {
		result = append(result, embedding.Values)
	}
	return result, nil
}
