package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/google/uuid"
)

// TestJWTService creates a JWTService with test configuration
func TestJWTService() *services.JWTService {
	return services.NewJWTService(
		"test-secret-key-for-testing-only",
		15*time.Minute,
		24*time.Hour,
	)
}

// GenerateTestToken generates a valid JWT token for testing
func GenerateTestToken(t *testing.T, userID uuid.UUID, email string) string {
	t.Helper()
	jwtSvc := TestJWTService()
	pair, err := jwtSvc.GenerateTokenPair(userID, email)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return pair.AccessToken
}

// AuthHeader returns an Authorization header value with a Bearer token
func AuthHeader(token string) string {
	return "Bearer " + token
}

// HTTPTestClient provides helper methods for HTTP testing
type HTTPTestClient struct {
	t       *testing.T
	handler http.Handler
}

// NewHTTPTestClient creates a new HTTP test client
func NewHTTPTestClient(t *testing.T, handler http.Handler) *HTTPTestClient {
	return &HTTPTestClient{t: t, handler: handler}
}

// Request makes an HTTP request and returns the response
func (c *HTTPTestClient) Request(method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	c.t.Helper()

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return rec
}

// GET makes a GET request
func (c *HTTPTestClient) GET(path string, headers map[string]string) *httptest.ResponseRecorder {
	return c.Request(http.MethodGet, path, nil, headers)
}

// POST makes a POST request
func (c *HTTPTestClient) POST(path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	return c.Request(http.MethodPost, path, body, headers)
}

// PATCH makes a PATCH request
func (c *HTTPTestClient) PATCH(path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	return c.Request(http.MethodPatch, path, body, headers)
}

// DELETE makes a DELETE request
func (c *HTTPTestClient) DELETE(path string, headers map[string]string) *httptest.ResponseRecorder {
	return c.Request(http.MethodDelete, path, nil, headers)
}

// ParseJSON parses the response body as JSON
func ParseJSON(t *testing.T, rec *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}
}

// AssertStatus asserts the response status code
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if rec.Code != expected {
		t.Errorf("expected status %d, got %d. Body: %s", expected, rec.Code, rec.Body.String())
	}
}

// AssertJSON asserts that the response contains expected JSON fields
func AssertJSON(t *testing.T, rec *httptest.ResponseRecorder, expected map[string]interface{}) {
	t.Helper()
	var actual map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&actual); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	for key, expectedVal := range expected {
		actualVal, ok := actual[key]
		if !ok {
			t.Errorf("expected key %q not found in response", key)
			continue
		}
		if expectedVal != actualVal {
			t.Errorf("expected %q=%v, got %v", key, expectedVal, actualVal)
		}
	}
}
