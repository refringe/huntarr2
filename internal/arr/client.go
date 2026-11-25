package arr

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxErrorBody is the maximum number of bytes read from a non-success response body for error messages.
const maxErrorBody = 512

// client communicates with a single *arr instance over its REST API.
type client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// newClient returns a client that never follows redirects and bounds every request at the transport level by ceiling.
func newClient(baseURL, apiKey string, ceiling time.Duration) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: ceiling,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// do sends a request carrying the instance's API key, with body as a JSON payload when it is non-nil.
func (c *client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", path, err)
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", path, err)
	}
	return resp, nil
}

// get executes a GET request against the given path and decodes the JSON response body into dst.
func (c *client) get(ctx context.Context, path string, dst any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return responseError(resp, path)
	}
	return decodeResponse(resp, path, dst)
}

// post executes a POST request against the given path with a JSON-encoded body, decoding the JSON response into dst.
func (c *client) post(ctx context.Context, path string, body, dst any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request body for %s: %w", path, err)
	}

	resp, err := c.do(ctx, http.MethodPost, path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return responseError(resp, path)
	}
	return decodeResponse(resp, path, dst)
}

// getBytes executes a GET request and returns up to maxBytes of the body with its content type; 404 yields ErrNotFound.
func (c *client) getBytes(ctx context.Context, path string, maxBytes int64) ([]byte, string, error) {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", fmt.Errorf("requesting %s: %w", path, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", responseError(resp, path)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("reading response from %s: %w", path, err)
	}

	return data, resp.Header.Get("Content-Type"), nil
}

// decodeResponse decodes the JSON response body into dst.
func decodeResponse(resp *http.Response, path string, dst any) error {
	if err := json.UnmarshalRead(resp.Body, dst); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// responseError returns an error describing the status code and a truncated snippet of the response body.
func responseError(resp *http.Response, path string) error {
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if len(snippet) > 0 {
		return fmt.Errorf("unexpected status %d from %s: %s",
			resp.StatusCode, path, bytes.TrimSpace(snippet))
	}
	return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
}
