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

// newClient returns a client for the given *arr instance: baseURL is trimmed of any trailing slash, redirects are
// never followed, and ceiling bounds every request at the transport level.
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

// get executes a GET request against the given path and decodes the JSON response body into dst.
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

	if err := json.UnmarshalRead(resp.Body, dst); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}

	return nil
}

// post executes a POST request against the given path with a JSON-encoded body, decoding the JSON response into dst.
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
