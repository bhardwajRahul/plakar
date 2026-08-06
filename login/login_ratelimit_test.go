package login

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// rateLimitBody mirrors the auth API payload from issue #1656: a 500 whose body
// carries the "limit-reached" marker instead of a 429 status code.
const rateLimitBody = `{"error":"OpError[limit-reached] Limit reached"}`

func TestRunWrapsErrRateLimitedFromBodyMarker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(rateLimitBody))
	}))
	defer srv.Close()

	flow := newTestFlow(t)
	flow.baseURL = srv.URL

	_, err := flow.Run("email", map[string]string{"email": "user@example.com"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want wrapping ErrRateLimited", err)
	}
}

func TestRunWrapsErrRateLimitedFromStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	flow := newTestFlow(t)
	flow.baseURL = srv.URL

	_, err := flow.Run("email", map[string]string{"email": "user@example.com"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want wrapping ErrRateLimited", err)
	}
}

func TestRunOtherErrorIsNotRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	flow := newTestFlow(t)
	flow.baseURL = srv.URL

	_, err := flow.Run("email", map[string]string{"email": "user@example.com"})
	if err == nil {
		t.Fatal("err = nil, want a non-nil error")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, unexpectedly wraps ErrRateLimited", err)
	}
}
