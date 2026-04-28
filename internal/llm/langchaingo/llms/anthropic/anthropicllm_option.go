package anthropic

import (
	"context"
	"net/http"
	"time"

	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/vertex"
	"golang.org/x/oauth2/google"
)

type options struct {
	model       string
	initOptions []option.RequestOption
}

type Option func(*options)

// WithHTTPClient allows setting a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithHTTPClient(client))
	}
}

// WithBaseURL allows setting a custom base URL.
func WithBaseURL(baseURL string) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithBaseURL(baseURL))
	}
}

// WithMaxRetries allows setting the maximum number of retries.
func WithMaxRetries(retries int) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithMaxRetries(retries))
	}
}

// WithMiddleware allows adding middleware to the client.
func WithMiddleware(middlewares ...option.Middleware) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithMiddleware(middlewares...))
	}
}

// WithHeader allows setting a custom header.
func WithHeader(key, value string) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithHeader(key, value))
	}
}

// WithAPIKey allows setting the API key.
func WithAPIKey(apiKey string) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithAPIKey(apiKey))
	}
}

// WithToken allows setting the authentication token.
func WithToken(token string) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithAuthToken(token))
	}
}

// WithRequestTimeout allows setting a timeout for requests.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithRequestTimeout(timeout))
	}
}

// WithEnvironmentProduction sets the environment to production.
func WithEnvironmentProduction() Option {
	return func(opts *options) {
		opts.initOptions = append(opts.initOptions, option.WithEnvironmentProduction())
	}
}

// WithModel passes the Anthropic model to the client.
func WithModel(model string) Option {
	return func(opts *options) {
		opts.model = model
	}
}

func WithGoogleCredentials(region, projectID string, cred *google.Credentials) Option {
	return func(opts *options) {
		opts.initOptions = append(
			opts.initOptions, vertex.WithCredentials(context.Background(), region, projectID, cred))
	}
}
