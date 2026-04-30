package genai

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"

	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/callbacks"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms"
	"github.com/shoucheng/my-first-agent/internal/llm/langchaingo/llms/googleai"
)

// Vertex is a type that represents a Gemini / Vertex AI API client.
//
// The unified google.golang.org/genai SDK supports both the Gemini
// Developer API (API key) and Vertex AI (project + location). Backend
// selection is based on whether CloudProject + CloudLocation are set.
type Vertex struct {
	CallbacksHandler callbacks.Handler
	client           *genai.Client
	opts             googleai.Options
}

var _ llms.Model = &Vertex{}

// New creates a new Vertex client.
func New(ctx context.Context, opts ...googleai.Option) (*Vertex, error) {
	clientOptions := googleai.DefaultOptions()
	for _, opt := range opts {
		opt(&clientOptions)
	}
	clientOptions.EnsureAuthPresent()

	cfg := &genai.ClientConfig{
		HTTPClient:  clientOptions.ClientOptionConf.HttpClient,
		HTTPOptions: clientOptions.HTTPOptions,
	}

	useVertex := clientOptions.CloudProject != "" && clientOptions.CloudLocation != ""
	if useVertex {
		cfg.Backend = genai.BackendVertexAI
		cfg.Project = clientOptions.CloudProject
		cfg.Location = clientOptions.CloudLocation

		// The genai SDK reads credentials via ADC. If credentials are
		// supplied explicitly, expose them through GOOGLE_APPLICATION_CREDENTIALS.
		if clientOptions.ClientOptionConf.CredentialsFile != "" {
			if _, err := os.Stat(clientOptions.ClientOptionConf.CredentialsFile); err != nil {
				return nil, fmt.Errorf("read credentials file failure, %w", err)
			}
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", clientOptions.ClientOptionConf.CredentialsFile)
		} else if len(clientOptions.ClientOptionConf.CredentialsJSON) > 0 {
			tmp, err := os.CreateTemp("", "googleai-cred-*.json")
			if err != nil {
				return nil, fmt.Errorf("create credentials temp file failure, %w", err)
			}
			if _, err := tmp.Write(clientOptions.ClientOptionConf.CredentialsJSON); err != nil {
				_ = tmp.Close()
				return nil, fmt.Errorf("write credentials temp file failure, %w", err)
			}
			if err := tmp.Close(); err != nil {
				return nil, fmt.Errorf("close credentials temp file failure, %w", err)
			}
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmp.Name())
		}
	} else {
		cfg.Backend = genai.BackendGeminiAPI
		cfg.APIKey = clientOptions.ClientOptionConf.ApiKey
	}

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Vertex{client: client, opts: clientOptions}, nil
}
