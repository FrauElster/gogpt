package gpt

type gptPromptResponse struct {
	Id      string `json:"id"`
	Object  string `json:"object"`
	Created int    `json:"created"`
	Model   string `json:"model"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
}

type gptBatchSingleRequest struct {
	CustomId string           `json:"custom_id"`
	Method   string           `json:"method"`
	Url      string           `json:"url"`
	Body     gptPromptRequest `json:"body"`
}

type gptBatchSingleResponse struct {
	ID       string `json:"id"`
	CustomId string `json:"custom_id"`
	Response struct {
		StatusCode int               `json:"status_code"`
		RequestID  string            `json:"request_id"`
		Body       gptPromptResponse `json:"body"`
	} `json:"response"`
}

type gptPromptRequest struct {
	Seed     int    `json:"seed,omitempty"`
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Temperature    float64           `json:"temperature"`
	ResponseFormat gptResponseFormat `json:"response_format"`
}

type gptResponseFormat struct {
	Type       string         `json:"type"`
	JsonSchema *gptJsonSchema `json:"json_schema,omitempty"`
}

type gptJsonSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type gptBatchRequest struct {
	InputFileId      string `json:"input_file_id"`
	Endpoint         string `json:"endpoint"`
	CompletionWindow string `json:"completion_window"`
}

type GptFilePurpose string

const (
	Assistants        GptFilePurpose = "assistants"
	Assistants_output GptFilePurpose = "assistants_output"
	Batch             GptFilePurpose = "batch"
	Batch_output      GptFilePurpose = "batch_output"
	FineTune          GptFilePurpose = "fine-tune"
	FineTuneResults   GptFilePurpose = "fine-tune-results"
	Vision            GptFilePurpose = "vision"
)

type GptFileStatus string

const (
	Uploaded  GptFileStatus = "uploaded"
	Processed GptFileStatus = "processed"
	Error     GptFileStatus = "error"
)

type GptFileResponse struct {
	ID        string         `json:"id"`
	Object    string         `json:"object"`
	Bytes     int            `json:"bytes"`
	CreatedAt int64          `json:"created_at"`
	Filename  string         `json:"filename"`
	Purpose   GptFilePurpose `json:"purpose"`
	Status    *GptFileStatus `json:"status,omitempty"`
}

type gptBatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
	Line    int    `json:"line"`
}

type GptBatchStatus string

const (
	BatchStatusInProgress GptBatchStatus = "in_progress"
	BatchStatusComplete   GptBatchStatus = "completed"
	BatchStatusFailed     GptBatchStatus = "failed"
	BatchStatusFinalizing GptBatchStatus = "finalizing"
)

type GptBatchResponse struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Endpoint string `json:"endpoint"`
	Errors   *struct {
		Object string          `json:"object"`
		Data   []gptBatchError `json:"data"`
	} `json:"errors"` // Using *string to allow null value
	InputFileID      string         `json:"input_file_id"`
	CompletionWindow string         `json:"completion_window"`
	Status           GptBatchStatus `json:"status"`
	OutputFileID     *string        `json:"output_file_id"`
	ErrorFileID      *string        `json:"error_file_id"`
	CreatedAt        int64          `json:"created_at"`
	InProgressAt     *int64         `json:"in_progress_at"`
	ExpiresAt        *int64         `json:"expires_at"`
	FinalizingAt     *int64         `json:"finalizing_at"`
	CompletedAt      *int64         `json:"completed_at"`
	FailedAt         *int64         `json:"failed_at"` // Using *int64 to allow null value
	ExpiredAt        *int64         `json:"expired_at"`
	CancellingAt     *int64         `json:"cancelling_at"`
	CancelledAt      *int64         `json:"cancelled_at"`
	RequestCounts    struct {
		Total     int `json:"total"`
		Completed int `json:"completed"`
		Failed    int `json:"failed"`
	} `json:"request_counts"`
	Metadata struct {
		CustomerID       string `json:"customer_id"`
		BatchDescription string `json:"batch_description"`
	} `json:"metadata"`
}

type gptFileDeletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}

type gptBatchesResponse struct {
	Object  string             `json:"object"`
	Data    []GptBatchResponse `json:"data"`
	HasMore bool               `json:"has_more"`
	FirstId string             `json:"first_id"`
	LastId  string             `json:"last_id"`
}

type gptFilesResponse struct {
	Object string            `json:"object"`
	Data   []GptFileResponse `json:"data"`
}
