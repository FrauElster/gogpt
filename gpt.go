package gpt

import (
	"context"
	"fmt"
	"net/http"

	"github.com/FrauElster/goerror"
)

var ErrGptAsk = goerror.New("gpt_ask", "Error while asking GPT")
var ErrInvalidContent = goerror.New("invalid_content", "Invalid content")

type Gpt struct {
	token string
	model string
	seed  int // https://platform.openai.com/docs/guides/text-generation/reproducible-outputs

	cacheDir string
	client   *http.Client
}

type Option func(*Gpt)

var WithModel = func(model string) Option {
	return func(g *Gpt) {
		if model != "" {
			g.model = model
		}
	}
}
var WithCacheDir = func(cacheDir string) Option { return func(g *Gpt) { g.cacheDir = cacheDir } }

func NewGpt(token string, opts ...Option) (*Gpt, error) {
	gzipTransport := &GzipRoundTripper{Transport: http.DefaultTransport}
	headerTransport := &HeaderRoundTripper{
		Transport: gzipTransport,
		headersToAdd: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", token),
			"Accept":        "application/json",
		},
	}
	backofTransport := NewBackoffRoundTripper(headerTransport)

	gpt := &Gpt{
		token:  token,
		model:  "gpt-4o",
		seed:   420,
		client: &http.Client{Transport: backofTransport},
	}

	for _, opt := range opts {
		opt(gpt)
	}

	return gpt, nil
}

func (g *Gpt) NewBatchSession() *GptBatchSession {
	return &GptBatchSession{
		seed:            g.seed,
		model:           g.model,
		client:          g.client,
		batches:         make(map[string]GptBatchResponse),
		files:           make(map[string][]byte),
		cacheDir:        g.cacheDir,
		createBatchData: make([]byte, 0),
	}
}

func (g *Gpt) RetrieveBatch(ctx context.Context, batchId string) (GptBatchResponse, goerror.TraceableError) {
	return retrieveBatch(ctx, g.client, batchId)
}

func (g *Gpt) RetrieveBatches(ctx context.Context, stati ...GptBatchStatus) ([]GptBatchResponse, goerror.TraceableError) {
	batches, err := retrieveBatches(ctx, g.client)
	if err != nil {
		return nil, err
	}
	if len(stati) == 0 {
		return batches, nil
	}
	batches = FilterSlice(batches, func(batch GptBatchResponse) bool {
		return SliceContains(stati, GptBatchStatus(batch.Status))
	})
	return batches, nil
}

func (g *Gpt) CancelBatch(ctx context.Context, batchId string) goerror.TraceableError {
	return cancelBatch(ctx, g.client, batchId)
}

func (g *Gpt) DeleteFile(ctx context.Context, fileId string) goerror.TraceableError {
	return deleteFile(ctx, g.client, fileId)
}

func (g *Gpt) RetrieveFileContent(ctx context.Context, fileId string) ([]byte, goerror.TraceableError) {
	return retrieveFileContent(ctx, g.client, fileId)
}

func (g *Gpt) RetrieveFiles(ctx context.Context) ([]GptFileResponse, goerror.TraceableError) {
	return retrieveFiles(ctx, g.client)
}

func (g *Gpt) RetrieveFile(ctx context.Context, fileId string) (GptFileResponse, goerror.TraceableError) {
	return retrieveFile(ctx, g.client, fileId)
}
