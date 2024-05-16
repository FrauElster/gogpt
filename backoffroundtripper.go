package gpt

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FrauElster/goerror"
)

var ErrTooManyRetries = goerror.New("too_many_retries", "too many retries")

// BackoffRoundTripper is a custom http.RoundTripper that limits the number of concurrent requests and applies backoff when receiving a 429 response.
type BackoffRoundTripper struct {
	base http.RoundTripper

	backoff        time.Duration
	backoffFactor  float64
	defaultBackoff time.Duration
	maxBackoff     time.Duration
	backoffMutex   sync.Mutex
	successSeq     int

	// retries limits the number of retries for a single request due to 429 responses
	// if set to 0, there is no limit
	retries int

	sequentialMode      atomic.Bool
	sequentialModeMutex sync.Mutex
}

func NewBackoffRoundTripper(transport http.RoundTripper) *BackoffRoundTripper {
	return &BackoffRoundTripper{
		base:           transport,
		defaultBackoff: 1 * time.Second,
		backoffFactor:  2,
		maxBackoff:     1 * time.Minute,
	}
}

func (rt *BackoffRoundTripper) RoundTrip(req *http.Request) (res *http.Response, err error) {
	counter := 0
	for {
		counter++
		if rt.retries > 0 && counter > rt.retries {
			err := fmt.Errorf("too many retries (%d) for a single request", rt.retries)
			return nil, ErrTooManyRetries.WithError(err)
		}

		if !rt.sequentialMode.Load() {
			res, err = rt.base.RoundTrip(req)
		} else {
			rt.sequentialModeMutex.Lock()

			rt.backoffMutex.Lock()
			backoff := rt.backoff
			rt.backoffMutex.Unlock()

			time.Sleep(backoff)
			res, err = rt.base.RoundTrip(req)

			rt.sequentialModeMutex.Unlock()
		}

		if err != nil {
			return res, err
		}

		rt.adjustModeAndBackoff(res.StatusCode)
		if res.StatusCode != http.StatusTooManyRequests {
			return res, err
		}
	}
}

func (rt *BackoffRoundTripper) adjustModeAndBackoff(statusCode int) {
	rt.backoffMutex.Lock()
	defer rt.backoffMutex.Unlock()

	if statusCode != http.StatusOK && statusCode != http.StatusTooManyRequests {
		// another error occurred, which should not effect the backoff
		return
	}

	if statusCode == http.StatusTooManyRequests {
		// backoff further
		rt.sequentialMode.Store(true)
		if rt.backoff < rt.defaultBackoff {
			rt.backoff = rt.defaultBackoff
		}

		rt.backoff = time.Duration(float64(rt.backoff) * rt.backoffFactor)
		if rt.backoff > rt.maxBackoff {
			rt.backoff = rt.maxBackoff
		}
		return
	}

	rt.successSeq++
	if rt.successSeq < 10 {
		// we want to have at least 10 successful requests in a row before decreasing the backoff
		return
	}

	// decrease backoff
	rt.backoff = time.Duration(float64(rt.backoff) / rt.backoffFactor)
	if rt.backoff < rt.defaultBackoff {
		// Switch back to concurrent mode
		rt.backoff = rt.defaultBackoff
		rt.sequentialMode.Store(false)
	}
}
