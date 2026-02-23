package handlers

import (
	"encoding/base64"
	"io"
	"strings"

	"github.com/dimitrije/nikode-api/internal/hub"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type WebhookHandler struct {
	hub *hub.Hub
}

func NewWebhookHandler(h *hub.Hub) *WebhookHandler {
	return &WebhookHandler{hub: h}
}

func (h *WebhookHandler) HandleIncoming(c *drift.Context) {
	// Extract subdomain from Host header (with proxy support)
	host := c.Request.Host
	if fwdHost := c.Request.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	subdomain := extractSubdomain(host)
	if subdomain == "" {
		c.BadRequest("invalid subdomain")
		return
	}

	// Check if tunnel exists
	_, ok := h.hub.GetTunnel(subdomain)
	if !ok {
		c.NotFound("tunnel not found")
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.BadRequest("failed to read body")
		return
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Create tunnel request
	req := &hub.TunnelRequest{
		ID:        uuid.New().String(),
		Subdomain: subdomain,
		Method:    c.Request.Method,
		Path:      c.Request.URL.RequestURI(),
		Headers:   headers,
		Body:      base64.StdEncoding.EncodeToString(body),
	}

	// Send request and wait for response
	resp, err := h.hub.SendTunnelRequest(subdomain, req)
	if err != nil {
		c.GatewayTimeout(err.Error())
		return
	}

	if resp.Error != "" {
		c.BadGateway(resp.Error)
		return
	}

	// Decode response body first so we know the actual length
	respBody, _ := base64.StdEncoding.DecodeString(resp.Body)

	// Headers to skip: hop-by-hop headers and Content-Length
	// (Content-Length will be set correctly by Go's HTTP server based on actual body size)
	skipHeaders := map[string]bool{
		"content-length":           true,
		"content-encoding":        true,
		"transfer-encoding":       true,
		"connection":              true,
		"keep-alive":              true,
		"proxy-authenticate":      true,
		"proxy-authorization":     true,
		"te":                      true,
		"trailer":                 true,
		"upgrade":                 true,
	}

	// Write response headers (excluding hop-by-hop and Content-Length)
	for key, value := range resp.Headers {
		if !skipHeaders[strings.ToLower(key)] {
			c.Response.Header().Set(key, value)
		}
	}

	// Write response body
	c.Response.WriteHeader(resp.StatusCode)
	_, _ = c.Response.Write(respBody)
	c.Abort()
}

func extractSubdomain(host string) string {
	// host: "abc123.webhooks.nikode.dimitrije.dev" or "abc123.webhook.nikode.dimitrije.dev:443"
	host = strings.Split(host, ":")[0]
	suffix := ".webhooks.nikode.dimitrije.dev"
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	return strings.TrimSuffix(host, suffix)
}
