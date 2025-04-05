package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
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

// sseTransport is a custom http.RoundTripper that intercepts and modifies SSE responses
type sseTransport struct {
	Transport  http.RoundTripper
	proxyHost  string
	targetHost string
}

func (t *sseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Call the underlying transport
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	
	// Check if this is an SSE response
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return resp, nil
	}
	
	log.Printf("INFO: Intercepting SSE response to modify endpoint events")
	
	// Create a response wrapper that modifies the response body
	originalBody := resp.Body
	pr, pw := io.Pipe()
	
	go func() {
		defer originalBody.Close()
		defer pw.Close()
		
		scanner := bufio.NewScanner(originalBody)
		for scanner.Scan() {
			line := scanner.Text()
			
			// Check if this line contains an endpoint event
			if strings.HasPrefix(line, "event: endpoint") {
				// Read the data line
				if scanner.Scan() {
					dataLine := scanner.Text()
					if strings.HasPrefix(dataLine, "data: ") {
						// Extract the endpoint URL
						endpoint := strings.TrimPrefix(dataLine, "data: ")
						
						// Replace the host in the endpoint
						log.Printf("DEBUG: Original endpoint: %s", endpoint)
						endpoint = strings.Replace(endpoint, t.targetHost, t.proxyHost, 1)
						log.Printf("DEBUG: Modified endpoint: %s", endpoint)
						
						// Write the modified event lines
						fmt.Fprintln(pw, line)
						fmt.Fprintln(pw, "data: "+endpoint)
						continue
					}
				}
			}
			
			// Write the original line for non-endpoint events
			fmt.Fprintln(pw, line)
		}
		
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading SSE stream: %v", err)
		}
	}()
	
	// Replace the response body with our modified pipe
	resp.Body = pr
	return resp, nil
}