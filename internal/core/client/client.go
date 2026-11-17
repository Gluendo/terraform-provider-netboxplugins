// Package client is the provider's NetBox REST client.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/shape"
)

// Config holds what the client needs to reach NetBox.
type Config struct {
	ServerURL          string
	APIToken           string
	AllowInsecureHTTPS bool
	RequestTimeout     time.Duration
	Headers            map[string]string
	UserAgent          string
}

// Client calls the NetBox REST API.
type Client struct {
	base      *url.URL
	auth      string
	headers   map[string]string
	userAgent string
	http      *http.Client
}

// ErrNotFound is returned when NetBox answers 404.
var ErrNotFound = errors.New("not found")

// APIError is a non-2xx answer from NetBox.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.Path, e.Status, formatErrorBody(e.Body))
}

// New builds a client.
func New(cfg Config) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(cfg.ServerURL, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("server_url %q is not an absolute URL", cfg.ServerURL)
	}
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.AllowInsecureHTTPS {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in by the user
	}
	return &Client{
		base:      u,
		auth:      authorization(cfg.APIToken),
		headers:   cfg.Headers,
		userAgent: cfg.UserAgent,
		http:      &http.Client{Timeout: timeout, Transport: tr},
	}, nil
}

// authorization builds the header for a v2 token (nbt_<key>.<secret>) or a
// v1 token.
func authorization(token string) string {
	if strings.HasPrefix(token, "nbt_") {
		return "Bearer " + token
	}
	return "Token " + token
}

// Get returns one object.
func (c *Client) Get(ctx context.Context, path string) (map[string]any, error) {
	return c.object(ctx, http.MethodGet, path, nil)
}

// Create posts a new object and returns it as NetBox stored it.
func (c *Client) Create(ctx context.Context, path string, payload map[string]any) (map[string]any, error) {
	return c.object(ctx, http.MethodPost, path, payload)
}

// Patch updates the given fields of an object and returns it.
func (c *Client) Patch(ctx context.Context, path string, payload map[string]any) (map[string]any, error) {
	return c.object(ctx, http.MethodPatch, path, payload)
}

// Delete removes an object.
func (c *Client) Delete(ctx context.Context, path string) error {
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// List returns the objects of a collection matching query, following
// pagination. With max > 0 it stops after max objects.
func (c *Client) List(ctx context.Context, path string, query url.Values, max int) ([]map[string]any, error) {
	q := url.Values{}
	for k, v := range query {
		q[k] = v
	}
	switch {
	case max > 0:
		q.Set("limit", strconv.Itoa(max))
	case q.Get("limit") == "":
		q.Set("limit", "250")
	}
	var out []map[string]any
	next := path + "?" + q.Encode()
	for next != "" {
		body, err := c.do(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		v, err := shape.Decode(body)
		if err != nil {
			return nil, err
		}
		page, _ := v.(map[string]any)
		results, _ := page["results"].([]any)
		for _, r := range results {
			if m, ok := r.(map[string]any); ok {
				out = append(out, m)
			}
		}
		next = ""
		if max > 0 && len(out) >= max {
			break
		}
		if n, ok := page["next"].(string); ok && n != "" {
			nu, err := url.Parse(n)
			if err != nil {
				return nil, err
			}
			next = nu.RequestURI()
		}
	}
	return out, nil
}

// Raw performs a request and returns the decoded body, for endpoints that
// are not plugin objects (status, custom fields).
func (c *Client) Raw(ctx context.Context, method, path string) (any, error) {
	body, err := c.do(ctx, method, path, nil)
	if err != nil {
		return nil, err
	}
	return shape.Decode(body)
}

func (c *Client) object(ctx context.Context, method, path string, payload map[string]any) (map[string]any, error) {
	body, err := c.do(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}
	v, err := shape.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s %s: expected an object, got %T", method, path, v)
	}
	return m, nil
}

const maxAttempts = 3

func (c *Client) do(ctx context.Context, method, path string, payload any) ([]byte, error) {
	target := *c.base
	ref, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	target.Path = c.base.Path + ref.Path
	target.RawQuery = ref.RawQuery

	var reqBody []byte
	if payload != nil {
		if reqBody, err = json.Marshal(payload); err != nil {
			return nil, err
		}
	}

	for attempt := 1; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", c.auth)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}

		tflog.Debug(ctx, "netbox request", map[string]any{"method": method, "path": target.Path, "attempt": attempt})
		resp, err := c.http.Do(req)
		if err != nil {
			if method == http.MethodGet && attempt < maxAttempts && ctx.Err() == nil {
				backoff(ctx, attempt)
				continue
			}
			return nil, fmt.Errorf("%s %s: %w", method, target.Path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		tflog.Debug(ctx, "netbox response", map[string]any{"method": method, "path": target.Path, "status": resp.StatusCode})

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return body, nil
		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("%s %s: %w", method, target.Path, ErrNotFound)
		case method == http.MethodGet && attempt < maxAttempts &&
			(resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout):
			backoff(ctx, attempt)
			continue
		}
		return nil, &APIError{Method: method, Path: target.Path, Status: resp.StatusCode, Body: string(body)}
	}
}

func backoff(ctx context.Context, attempt int) {
	t := time.NewTimer(time.Duration(attempt) * time.Second)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// formatErrorBody turns NetBox's {"field": ["message"]} errors into one line.
func formatErrorBody(body string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil || len(m) == 0 {
		if len(body) > 500 {
			return body[:500] + "..."
		}
		return body
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %v", k, m[k]))
	}
	return strings.Join(parts, "; ")
}
