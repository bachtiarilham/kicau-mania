// Package client implements authenticated engine-to-cloud HTTP requests.
package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"kicau.local/edge/internal/infra/logger"
)

const maxResponseBytes = 2 * 1024 * 1024

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

// Error contains only safe transport metadata, never URLs, credentials, or bodies.
// Kind is one of context, endpoint, encoding, request_id, transport, status,
// response, or rejected. RequestID is the validated ID returned by cloud.
type Error struct {
	Kind       string
	StatusCode int
	RequestID  string
	cause      error
}

func (e *Error) Error() string {
	return fmt.Sprintf("cloud request failed: kind=%s status=%d request_id=%s", e.Kind, e.StatusCode, e.RequestID)
}

func (e *Error) Unwrap() error { return e.cause }

func failure(ctx context.Context, kind string, status int, requestID string, err error) error {
	var cause error
	// Preserve cancellation without retaining a url.Error or remote error text.
	switch {
	case ctx.Err() != nil:
		cause = ctx.Err()
	case errors.Is(err, context.Canceled):
		cause = context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		cause = context.DeadlineExceeded
	}
	return &Error{Kind: kind, StatusCode: status, RequestID: requestID, cause: cause}
}

type Client struct {
	endpoint, token string
	http            *http.Client
}

func New(endpoint, token string) *Client {
	return &Client{endpoint: endpoint, token: token, http: &http.Client{
		Timeout:       8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *Client) Post(ctx context.Context, body, out any) error {
	return c.request(ctx, http.MethodPost, "", body, out)
}

// request is the only HTTP/JSON implementation for all engine-to-cloud calls.
func (c *Client) request(ctx context.Context, method, suffix string, body, out any) error {
	if err := ctx.Err(); err != nil {
		return failure(ctx, "context", 0, "", err)
	}
	endpoint, err := url.Parse(c.endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.Fragment != "" {
		return failure(ctx, "endpoint", 0, "", nil)
	}
	if suffix != "" {
		endpoint.Path = strings.TrimRight(endpoint.Path, "/") + suffix
		if endpoint.RawPath != "" {
			endpoint.RawPath = strings.TrimRight(endpoint.RawPath, "/") + suffix
		}
	}
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return failure(ctx, "encoding", 0, "", err)
		}
		payload = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), payload)
	if err != nil {
		return failure(ctx, "endpoint", 0, "", err)
	}
	requestID := logger.RequestID(ctx)
	if !requestIDPattern.MatchString(requestID) {
		var value [16]byte
		if _, err := rand.Read(value[:]); err != nil {
			return failure(ctx, "request_id", 0, "", err)
		}
		requestID = hex.EncodeToString(value[:])
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return failure(ctx, "transport", 0, "", err)
	}
	defer res.Body.Close()
	remoteID := res.Header.Get("X-Request-ID")
	if !requestIDPattern.MatchString(remoteID) {
		remoteID = ""
	}
	if res.StatusCode != http.StatusOK {
		return failure(ctx, "status", res.StatusCode, remoteID, nil)
	}
	mediaType, _, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return failure(ctx, "response", res.StatusCode, remoteID, nil)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return failure(ctx, "response", res.StatusCode, remoteID, err)
	}
	// Decode exactly one JSON document. Unknown fields allow additive contracts.
	var envelope struct {
		Success *bool           `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Success == nil {
		return failure(ctx, "response", res.StatusCode, remoteID, nil)
	}
	if !*envelope.Success {
		return failure(ctx, "rejected", res.StatusCode, remoteID, nil)
	}
	if len(envelope.Data) == 0 || bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("null")) {
		return failure(ctx, "response", res.StatusCode, remoteID, nil)
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return failure(ctx, "response", res.StatusCode, remoteID, nil)
	}
	return nil
}
