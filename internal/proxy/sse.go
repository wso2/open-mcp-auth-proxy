package proxy

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

// HandleSSE sets up a go-routine to wait for context cancellation
// and flushes the response if possible.
func HandleSSE(w http.ResponseWriter, r *http.Request, rp *httputil.ReverseProxy) {
	ctx := r.Context()
	done := make(chan struct{})

	go func() {
		<-ctx.Done()
		log.Printf("INFO: SSE connection closed from %s (path: %s)", r.RemoteAddr, r.URL.Path)
		close(done)
	}()

	rp.ServeHTTP(w, r)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	<-done
}

// NewShutdownContext is a little helper to gracefully shut down
func NewShutdownContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
