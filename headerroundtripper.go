package gpt

import "net/http"

// HeaderRoundTripper wraps an http.RoundTripper, adding specific headers
type HeaderRoundTripper struct {
	Transport    http.RoundTripper
	headersToAdd map[string]string
}

// RoundTrip executes a single HTTP transaction and adds custom headers
func (h *HeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original request
	clonedReq := req.Clone(req.Context())

	// Add custom headers to the cloned request
	for key, value := range h.headersToAdd {
		clonedReq.Header.Add(key, value)
	}

	// Use the underlying RoundTripper to execute the modified request
	return h.Transport.RoundTrip(clonedReq)
}
