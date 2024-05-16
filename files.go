package gpt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/FrauElster/goerror"
)

var (
	ErrRequestFile = goerror.New("gpt:request_file", "failed to request files")
	ErrDeleteFile  = goerror.New("gpt:delete_file", "Failed to delete file")
	ErrCreateFile  = goerror.New("gpt:create_file", "Failed to create file")
)

func deleteFile(ctx context.Context, c *http.Client, fileId string) goerror.TraceableError {
	url := fmt.Sprintf("https://api.openai.com/v1/files/%s", fileId)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create request: %w", err)
		return ErrDeleteFile.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)

	resp, err := c.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to request deletion: %w", err)
		return ErrDeleteFile.WithError(err).WithOrigin()
	}
	if resp.StatusCode == http.StatusNotFound {
		// probably already deleted
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("server responded with non-OK status (%s)", resp.Status)
		return ErrDeleteFile.WithError(err).WithOrigin()
	}

	data, err := parseResponse[gptFileDeletionResponse](resp)
	if err != nil {
		return ErrDeleteFile.WithError(err).WithOrigin()
	}
	if !data.Deleted {
		return ErrDeleteFile.WithError(fmt.Errorf("file was not deleted")).WithOrigin()
	}

	return nil
}

func retrieveFiles(ctx context.Context, c *http.Client) ([]GptFileResponse, goerror.TraceableError) {
	url := "https://api.openai.com/v1/files"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}

	decodedResponse, err := parseResponse[gptFilesResponse](resp)
	if err != nil {
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}

	return decodedResponse.Data, nil
}

func retrieveFile(ctx context.Context, c *http.Client, fileId string) (GptFileResponse, goerror.TraceableError) {
	url := fmt.Sprintf("https://api.openai.com/v1/files/%s", fileId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return GptFileResponse{}, ErrRequestFile.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		return GptFileResponse{}, ErrRequestFile.WithError(err).WithOrigin()
	}
	decodedResponse, err := parseResponse[GptFileResponse](resp)
	if err != nil {
		return GptFileResponse{}, ErrRequestFile.WithError(err).WithOrigin()
	}

	return decodedResponse, nil
}

func retrieveFileContent(ctx context.Context, c *http.Client, fileId string) ([]byte, goerror.TraceableError) {
	url := "https://api.openai.com/v1/files/" + fileId + "/content"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create request: %w", err)
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	resp, err := c.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to request: %w", err)
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("server responded with non-OK status (%s)", resp.Status)
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("could not decode body: %w", err)
		return nil, ErrRequestFile.WithError(err).WithOrigin()
	}

	return data, nil
}

func uploadBatchFile(ctx context.Context, c *http.Client, filename string, data []byte) (string, goerror.TraceableError) {
	// Create a buffer to write our multipart/form-data to
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	// Add the JSONL file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		err = fmt.Errorf("failed to create form file: %w", err)
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}
	_, err = part.Write(data)
	if err != nil {
		err = fmt.Errorf("failed to write form file: %w", err)
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}
	// Add the purpose field
	err = writer.WriteField("purpose", "batch")
	if err != nil {
		err = fmt.Errorf("failed to write field: %w", err)
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}
	err = writer.Close()
	if err != nil {
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}

	// Create the request
	url := "https://api.openai.com/v1/files"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}
	req = req.WithContext(ctx)
	req.Header = http.Header{"Content-Type": []string{writer.FormDataContentType()}}

	// Send the request
	resp, err := c.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to upload batch file: %w", err)
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}

	decodedResponse, err := parseResponse[GptFileResponse](resp)
	if err != nil {
		return "", ErrCreateFile.WithError(err).WithOrigin()
	}

	return decodedResponse.ID, nil
}
