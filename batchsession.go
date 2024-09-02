package gpt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"reflect"
	"time"

	"github.com/FrauElster/goerror"
	"github.com/invopop/jsonschema"
)

var (
	ErrBatchFailed           = goerror.New("gpt:batch_failed", "Batch failed")
	ErrBatchNotCompleted     = goerror.New("gpt:batch_not_completed", "Batch not completed yet")
	ErrSerializeBatchRequest = goerror.New("gpt:serialize_batch_request", "Failed to serialize batch request")
	ErrExceedsFileLimit      = goerror.New("gpt:exceeds_file_limit", "Exceeds file limit")
	ErrParseBatchLine        = goerror.New("gpt:parse_batch_line", "failed to parse batch line")
)

type GptBatchSession struct {
	client *http.Client
	model  string
	seed   int // https://platform.openai.com/docs/guides/text-generation/reproducible-outputs

	// in-memory per session caches for retrieval
	batches map[string]GptBatchResponse
	files   map[string][]byte

	// for creation
	createBatchData []byte
	requestCount    int

	cacheDir string
}

type appliedRequestOption struct {
	model          string
	seed           int
	responseFormat gptResponseFormat
}

type RequestOption func(*appliedRequestOption) error

// WithJsonSchema adds a JSON schema to the request as response format.
// v must be a struct or pointer to a struct.
var WithJsonSchema = func(v any) RequestOption {
	return func(a *appliedRequestOption) error {
		schema, err := getJsonSchema(v)
		if err != nil {
			return goerror.New("gpt:json_schema", "failed to get json schema").WithError(err).WithOrigin()
		}

		a.responseFormat.Type = "json_schema"
		a.responseFormat.JsonSchema = &gptJsonSchema{
			Name:   getStructName(v),
			Strict: true,
			Schema: schema,
		}

		return nil
	}
}

// AddToBatch adds a request to the current batch data.
// The customRequestId is used to identify the request in the batch.
// It should have an application wide prefix to avoid collisions with other applications that batch data.
// Further the customRequestId should be unique, a timestamp is a good choice.
// The systemPrompt should describe the task and the userPrompt should contain the input data.
// The callee is responsible for store the lineIdx of the request.
// If AddToBatch is called the third time, the lineIdx of the request within the current batch is 2.
// If the batch data exceeds the 512MB limit, ErrExceedsFileLimit is returned,
// signaling that the s.CreateBatch() should be called to flush the current batch data
func (s *GptBatchSession) AddToBatch(customRequestId, systemPrompt, userPrompt string, options ...RequestOption) goerror.TraceableError {
	opts := &appliedRequestOption{
		model:          s.model,
		seed:           s.seed,
		responseFormat: gptResponseFormat{Type: "json_object"},
	}
	for _, opt := range options {
		if err := opt(opts); err != nil {
			return goerror.New("gpt:add_to_batch", "failed to apply option").WithError(err).WithOrigin()
		}
	}
	req := gptBatchSingleRequest{
		CustomId: customRequestId,
		Method:   "POST",
		Url:      "/v1/chat/completions",
		Body: gptPromptRequest{
			Model: opts.model,
			Seed:  opts.seed,
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			Temperature:    0,
			ResponseFormat: opts.responseFormat,
		},
	}

	if s.requestCount >= 50000 {
		return ErrExceedsFileLimit.WithError(errors.New("exceeds request limit")).WithOrigin()
	}

	serialized, err := json.Marshal(req)
	if err != nil {
		return ErrSerializeBatchRequest.WithError(err).WithOrigin()
	}
	serialized = append(serialized, '\n')

	exceedsFileLimit := len(s.createBatchData)+len(serialized) > 512*1024*1024+1
	if exceedsFileLimit {
		return ErrExceedsFileLimit.WithOrigin()
	}

	s.createBatchData = append(s.createBatchData, serialized...)
	s.requestCount++
	return nil
}

// CreateBatch creates a new batch with the current batch data.
// The batchName is used to identify the batch. It is the prefix for the file created and the batch created.
// the batchname should be unique to this application, to differentiate between different batches of different applications.
// CreateBatch will not clear its data. Create a new session to start a new batch.
func (s *GptBatchSession) CreateBatch(ctx context.Context, batchName string) (string, goerror.TraceableError) {
	if len(s.createBatchData) == 0 {
		return "", nil
	}

	filename := fmt.Sprintf("%s-%s.jsonl", batchName, time.Now().Format("2006-01-02T15-04-05"))
	fileId, err := uploadBatchFile(ctx, s.client, filename, s.createBatchData)
	if err != nil {
		return "", err
	}

	batchId, err := uploadBatch(ctx, s.client, fileId)
	if err != nil {
		return "", err
	}

	return batchId, nil
}

// RetrieveBatchedRequest retrieves a single request from a batch.
// The batchId is the id of the batch to retrieve the request from.
// The lineIdx is the index of the request in the batch.
// If the batch is not completed yet, ErrBatchNotCompleted is returned, which is more a flag indicating that the request should be retried later.
// If the batch failed, ErrBatchFailed is returned, which contains the error that caused the batch to fail.
// RetrieveBatchedRequest returns the raw []byte of the answer GPT gave (respnse.Body.Choices[0].Message.Content), since it is agnostic to the response format (could be JSON, could be plain text).
// Sometimes GPT messes up, and a JSONL line is malformed. In this case ErrParseBatchLine is returned.
// If you want to see the file itself causing that, just add a WithCacheDir to GPT instance and the file will be stored in the cache directory.
func (s *GptBatchSession) RetrieveBatchedRequest(ctx context.Context, batchId string, lineIdx int) ([]byte, goerror.TraceableError) {
	batch, err := s.getBatch(ctx, batchId)
	if err != nil {
		return nil, err
	}

	if batch.Status == "failed" {
		var batchErr error
		if batch.Errors != nil && len(batch.Errors.Data) > 0 {
			asErrs := MapSlice(batch.Errors.Data, func(e gptBatchError) error { return fmt.Errorf("%s: %s", e.Code, e.Message) })
			batchErr = errors.Join(asErrs...)
		}

		return nil, ErrBatchFailed.WithError(batchErr).WithOrigin()
	}

	if batch.Status != "completed" {
		return nil, ErrBatchNotCompleted.WithOrigin()
	}

	if batch.OutputFileID == nil {
		return nil, ErrRequestBatch.WithError(errors.New("no output file ID")).WithOrigin()
	}
	file, err := s.getFile(ctx, *batch.OutputFileID)
	if err != nil {
		return nil, err
	}

	lines := bytes.Split(file, []byte("\n"))
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, ErrParseBatchLine.WithError(fmt.Errorf("line index out of bounds[0,%d]: %d", len(lines)-1, lineIdx)).WithOrigin()
	}

	rawResponse := lines[lineIdx]
	var response gptBatchSingleResponse

	if err := json.Unmarshal(rawResponse, &response); err != nil {
		err = fmt.Errorf("failed to decode response: %w", err)
		return nil, ErrParseBatchLine.WithError(err).WithOrigin()
	}
	if response.Response.StatusCode != http.StatusOK {
		return nil, ErrParseBatchLine.WithError(fmt.Errorf("server responded with non-OK status: %d", response.Response.StatusCode)).WithOrigin()
	}
	if len(response.Response.Body.Choices) == 0 {
		return nil, ErrParseBatchLine.WithError(errors.New("gpt is clueless")).WithOrigin()
	}

	return []byte(response.Response.Body.Choices[0].Message.Content), nil
}

func (s *GptBatchSession) getBatch(ctx context.Context, batchId string) (GptBatchResponse, goerror.TraceableError) {
	// check session cache
	if batch, ok := s.batches[batchId]; ok {
		return batch, nil
	}

	var result GptBatchResponse
	// check persistent cache
	loadFromPersistentCache := func() error {
		filepath := path.Join(s.cacheDir, batchId+".json")
		_, err := os.Stat(filepath)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filepath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, &result)
		if err != nil {
			return err
		}
		return nil
	}
	if s.cacheDir != "" {
		err := loadFromPersistentCache()
		if err == nil {
			s.batches[batchId] = result
			return result, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			slog.Error("Failed to read batch from cache", "error", err, "batchId", batchId)
		}
	}

	var err goerror.TraceableError
	result, err = retrieveBatch(ctx, s.client, batchId)
	if err != nil {
		return result, err
	}

	// fill session cache
	s.batches[batchId] = result

	// fill persisten cache if completed
	if s.cacheDir != "" && result.Status == BatchStatusComplete {
		filepath := path.Join(s.cacheDir, batchId+".json")
		data, _ := json.Marshal(result)
		err := os.WriteFile(filepath, data, 0644)
		if err != nil {
			slog.Error("Failed to write batch to cache", "error", err, "batchId", batchId)
		}
	}

	return result, nil
}

func (s *GptBatchSession) getFile(ctx context.Context, fileId string) ([]byte, goerror.TraceableError) {
	// check session cache
	if file, ok := s.files[fileId]; ok {
		return file, nil
	}

	var data []byte

	// check persistent cache
	loadFromPersistentCache := func() error {
		filepath := path.Join(s.cacheDir, fileId+".jsonl")
		_, err := os.Stat(filepath)
		if err != nil {
			return err
		}
		data, err = os.ReadFile(filepath)
		if err != nil {
			return err
		}
		return nil
	}
	if s.cacheDir != "" {
		err := loadFromPersistentCache()
		if err == nil {
			s.files[fileId] = data
			return data, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			slog.Error("Failed to read file from cache", "error", err, "fileId", fileId)
		}
	}

	var err goerror.TraceableError
	data, err = retrieveFileContent(ctx, s.client, fileId)
	if err != nil {
		return nil, err
	}

	s.files[fileId] = data

	if s.cacheDir != "" {
		filepath := path.Join(s.cacheDir, fileId+".jsonl")
		err := os.WriteFile(filepath, data, 0644)
		if err != nil {
			slog.Error("Failed to write file to cache", "error", err, "fileId", fileId)
		}
	}
	return data, nil
}

func getJsonSchema(v any) (map[string]any, error) {
	if v == nil {
		return nil, fmt.Errorf("input must be a struct or a non-nil pointer to a struct")
	}
	if reflect.ValueOf(v).Kind() != reflect.Struct && reflect.ValueOf(v).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("input must be a struct or a non-nil pointer to a struct")
	}
	if reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("input must be a struct or a non-nil pointer to a struct")
	}

	// Generate the schema using the jsonschema package
	schemaReflector := jsonschema.Reflector{}
	schema := schemaReflector.Reflect(v)

	// Convert the schema to JSON
	schemaJSON, err := schema.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema to JSON: %w", err)
	}

	var schemaMap map[string]any

	err = json.Unmarshal(schemaJSON, &schemaMap)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to map: %w", err)
	}

	return schemaMap, nil
}

func getStructName(v any) string {
	// Get the reflect.Type of the input
	t := reflect.TypeOf(v)

	// If it's a pointer, get the element type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Ensure that the type is a struct
	if t.Kind() != reflect.Struct {
		return ""
	}

	// Return the name of the struct
	return t.Name()
}
