package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func parseResponse[T any](res *http.Response) (val T, err error) {
	// decode body
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return val, fmt.Errorf("could not decode body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return val, fmt.Errorf("server responded with non-OK status (%s): %s", res.Status, string(body))
	}

	// parse body
	err = json.Unmarshal(body, &val)
	if err != nil {
		return val, fmt.Errorf("could not parse response: %w", err)
	}

	return val, nil
}

func RequestToCurl(req *http.Request) string {
	var cmd bytes.Buffer
	cmd.WriteString("curl -X ")
	cmd.WriteString(req.Method)

	// Add headers to the curl command
	for key, values := range req.Header {
		if key == "Accept-Encoding" {
			continue
		}
		for _, value := range values {
			cmd.WriteString(fmt.Sprintf(" -H '%s: %s'", key, value))
		}
	}

	// Include the body in the curl command if present
	if req.Body != nil && req.Method != "GET" && req.Method != "HEAD" {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Printf("Error reading request body: %s\n", err)
			return ""
		}
		// Reset the req.Body so it can be read again later if needed
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		bodyString := strings.ReplaceAll(string(bodyBytes), "'", "'\\''") // Handle single quotes in the body
		cmd.WriteString(fmt.Sprintf(" -d '%s'", bodyString))
	}

	// Add the URL to the curl command
	cmd.WriteString(fmt.Sprintf(" '%s'", req.URL))

	return cmd.String()
}

func MapSlice[I, O any](slice []I, f func(I) O) []O {
	result := make([]O, len(slice))
	for i := range slice {
		result[i] = f(slice[i])
	}
	return result
}

func FilterSlice[I any](slice []I, f func(I) bool) []I {
	result := make([]I, 0, len(slice))
	for _, i := range slice {
		if f(i) {
			result = append(result, i)
		}
	}
	return result
}

func SliceContains[I comparable](slice []I, value I) bool {
	for _, i := range slice {
		if i == value {
			return true
		}
	}
	return false
}
