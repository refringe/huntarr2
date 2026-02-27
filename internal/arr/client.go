package arr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxErrorBody is the maximum number of bytes read from a non-success
// response body for inclusion in error messages.
const maxErrorBody = 512

// client communicates with a single *arr instance over its REST API.
type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// newClient returns a client configured for the given *arr instance. The
// trailing slash on baseURL is trimmed if present. Redirects are not
// followed to prevent SSRF via open redirects.
func newClient(baseURL, apiKey string, timeout time.Duration) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// get executes a GET request against the given path and decodes the JSON
// response body into dst.
func (c *client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck // best-effort body drain
		resp.Body.Close()              //nolint:errcheck // best-effort close
	}()

	if resp.StatusCode != http.StatusOK {
		return responseError(resp, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}

	return nil
}

// post executes a POST request against the given path with a JSON-encoded
// body and decodes the JSON response into dst.
func (c *client) post(ctx context.Context, path string, body, dst any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request body for %s: %w", path, err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded),
	)
	if err != nil {
		return fmt.Errorf("building request for %s: %w", path, err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body) //nolint:errcheck // best-effort body drain
		resp.Body.Close()              //nolint:errcheck // best-effort close
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return responseError(resp, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}

	return nil
}

// responseError reads a truncated snippet of the response body and returns
// an error describing the unexpected status code and any message from the
// server.
func responseError(resp *http.Response, path string) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if len(snippet) > 0 {
		return fmt.Errorf("unexpected status %d from %s: %s",
			resp.StatusCode, path, bytes.TrimSpace(snippet))
	}
	return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
}
