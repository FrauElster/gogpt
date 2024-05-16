package gpt

import (
	"compress/gzip"
	"io"
	"net/http"
)

// GzipRoundTripper wraps an http.RoundTripper, adding gzip compression support.
type GzipRoundTripper struct {
	Transport http.RoundTripper
}

// RoundTrip executes a single HTTP transaction and handles gzip compression.
func (g *GzipRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if g.Transport == nil {
		g.Transport = http.DefaultTransport
	}

	// Add the gzip header if not already included
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	// Perform the request
	resp, err := g.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Check if the response is gzip encoded
	if resp.Header.Get("Content-Encoding") == "gzip" {
		// Replace the body with a gzip reader
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}

		// Wrap the gzip reader so the Close method also closes the original body
		resp.Body = &gzipResponseReader{Reader: gzReader, Closer: resp.Body}

		// Remove the Content-Encoding header
		resp.Header.Del("Content-Encoding")
	}

	return resp, nil
}

// gzipResponseReader wraps the gzip reader and the original response body so
// that closing the reader will close the original body as well.
type gzipResponseReader struct {
	io.Reader
	Closer io.Closer
}

// Close closes the gzip reader and the original response body.
func (w *gzipResponseReader) Close() error {
	_ = w.Reader.(*gzip.Reader).Close()
	return w.Closer.Close()
}
