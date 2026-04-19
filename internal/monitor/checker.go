package monitor

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Checker is the interface that wraps the Check method.
// It allows different types of monitors (HTTP, TCP, Ping, etc.) to be used
// interchangeably by the Manager orchestration loop.
type Checker interface {
	Check(ctx context.Context) (statusCode *int, responseTimeMs int, isUp bool)
}

// HTTPChecker is the default implementation of Checker for HTTP/HTTPS endpoints.
type HTTPChecker struct {
	url    string
	client *http.Client
}

// NewHTTPChecker creates a new HTTPChecker for the given URL.
func NewHTTPChecker(url string) *HTTPChecker {
	return &HTTPChecker{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// Don't follow redirects automatically, just report the status code.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Check performs the HTTP check (trying HEAD first, falling back to GET).
func (c *HTTPChecker) Check(ctx context.Context) (*int, int, bool) {
	start := time.Now()
	statusCode, err := c.doRequest(ctx, c.url, http.MethodHead)
	if err != nil {
		// If HEAD fails, try GET as a fallback in case the server doesn't support HEAD.
		if statusCode != nil && *statusCode == http.StatusMethodNotAllowed {
			start = time.Now()
			statusCode, err = c.doRequest(ctx, c.url, http.MethodGet)
		}
	}
	responseTimeMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return nil, responseTimeMs, false
	}
	isUp := *statusCode >= 200 && *statusCode < 400
	return statusCode, responseTimeMs, isUp
}

func (c *HTTPChecker) doRequest(ctx context.Context, url, method string) (*int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	// Add a User-Agent so we don't get blocked by WAFs like Cloudflare
	req.Header.Set("User-Agent", "TinyPulse/1.0 (https://github.com/AkaCoder404/tinypulse)")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	return &code, nil
}

// TCPChecker attempts to open a TCP connection to a host:port.
type TCPChecker struct {
	address string
}

// NewTCPChecker creates a new TCPChecker. The address must include a port (e.g. "1.1.1.1:53")
func NewTCPChecker(address string) *TCPChecker {
	return &TCPChecker{address: address}
}

// Check performs the TCP connection check.
func (c *TCPChecker) Check(ctx context.Context) (*int, int, bool) {
	start := time.Now()

	// We use a Dialer to respect the context timeout
	dialer := net.Dialer{}
	
	conn, err := dialer.DialContext(ctx, "tcp", c.address)
	responseTimeMs := int(time.Since(start).Milliseconds())

	if err != nil {
		return nil, responseTimeMs, false
	}

	// If we successfully connected, close it immediately
	conn.Close()

	// TCP doesn't have HTTP status codes, so we just return nil for the code
	return nil, responseTimeMs, true
}
