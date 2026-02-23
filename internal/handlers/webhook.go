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
		c.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
		c.Response.WriteHeader(404)
		_, _ = c.Response.Write([]byte(tunnelNotFoundHTML))
		c.Abort()
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

const tunnelNotFoundHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Tunnel Not Found</title>
<link rel="icon" type="image/svg+xml" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' width='512' height='512' viewBox='0 0 512 512'><rect width='512' height='512' rx='80' ry='80' fill='%23374151'/><text x='256' y='380' font-family='Arial,Helvetica,sans-serif' font-size='360' font-weight='bold' fill='%23f3f4f6' text-anchor='middle'>N</text></svg>">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { min-height: 100vh; display: flex; flex-direction: column; align-items: center; justify-content: center; background: #111827; color: #f3f4f6; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
  h1 { font-size: 2rem; margin-bottom: 1.5rem; color: #9ca3af; }
  img { max-width: 480px; width: 100%; border-radius: 8px; }
</style>
</head>
<body>
  <h1>Tunnel Not Found</h1>
  <img src="https://media1.tenor.com/m/t-rN1agiUf0AAAAC/confused-john-travolta.gif" alt="Confused John Travolta">
</body>
</html>`

func extractSubdomain(host string) string {
	// host: "abc123.webhooks.nikode.dimitrije.dev" or "abc123.webhook.nikode.dimitrije.dev:443"
	host = strings.Split(host, ":")[0]
	suffix := ".webhooks.nikode.dimitrije.dev"
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	return strings.TrimSuffix(host, suffix)
}
