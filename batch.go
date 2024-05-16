package gpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FrauElster/goerror"
)

var (
	ErrRequestBatch = goerror.New("gpt:retrieve_batch", "Failed to retrieve batch")
	ErrCancelBatch  = goerror.New("gpt:cancel_batch", "Failed to cancel batch")
	ErrCreateBatch  = goerror.New("gpt:create_batch", "Failed to create batch")
)

func retrieveBatches(ctx context.Context, c *http.Client) ([]GptBatchResponse, goerror.TraceableError) {
	batches := make([]GptBatchResponse, 0)

	var cursor string
	for {
		url := "https://api.openai.com/v1/batches&limit=100"
		if cursor != "" {
			url += "&after=" + cursor
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, ErrRequestBatch.WithError(err).WithOrigin()
		}
		req = req.WithContext(ctx)
		resp, err := c.Do(req)
		if err != nil {
			return nil, ErrRequestBatch.WithError(err).WithOrigin()
		}
		decodedResponse, err := parseResponse[gptBatchesResponse](resp)
		if err != nil {
			return nil, ErrRequestBatch.WithError(err).WithOrigin()
		}

		batches = append(batches, decodedResponse.Data...)

		if !decodedResponse.HasMore {
			break
		}
		cursor = decodedResponse.LastId
	}

	return batches, nil
}

func retrieveBatch(ctx context.Context, c *http.Client, batchId string) (GptBatchResponse, goerror.TraceableError) {
	url := "https://api.openai.com/v1/batches/" + batchId
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return GptBatchResponse{}, ErrRequestBatch.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return GptBatchResponse{}, ErrRequestBatch.WithError(err).WithOrigin()
	}
	decodedResponse, err := parseResponse[GptBatchResponse](resp)
	if err != nil {
		return GptBatchResponse{}, ErrRequestBatch.WithError(err).WithOrigin()
	}

	return decodedResponse, nil
}

func cancelBatch(ctx context.Context, c *http.Client, batchId string) goerror.TraceableError {
	url := "https://api.openai.com/v1/batches/" + batchId + "/cancel"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return ErrGptAsk.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return ErrGptAsk.WithError(err).WithOrigin()
	}
	if resp.StatusCode == http.StatusConflict {
		// already cancelled
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return ErrCancelBatch.WithError(fmt.Errorf("failed to cancel batch: %s", resp.Status)).WithOrigin()
	}
	return nil
}

func uploadBatch(ctx context.Context, c *http.Client, fileId string) (string, goerror.TraceableError) {
	body := gptBatchRequest{
		InputFileId:      fileId,
		Endpoint:         "/v1/chat/completions",
		CompletionWindow: "24h",
	}
	serializedBody, err := json.Marshal(body)
	if err != nil {
		err = fmt.Errorf("failed to serialize body: %w", err)
		return "", ErrCreateBatch.WithError(err).WithOrigin()
	}

	url := "https://api.openai.com/v1/batches"
	req, err := http.NewRequest("POST", url, bytes.NewReader(serializedBody))
	if err != nil {
		return "", ErrCreateBatch.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	req.Header = http.Header{"Content-Type": {"application/json"}}

	resp, err := c.Do(req)
	if err != nil {
		return "", ErrCreateBatch.WithError(err).WithOrigin()
	}

	decodedResponse, err := parseResponse[GptBatchResponse](resp)
	if err != nil {
		return "", ErrCreateBatch.WithError(err).WithOrigin()
	}

	return decodedResponse.ID, nil
}
