package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Zoom posts to a Zoom Team Chat "Incoming Webhook" endpoint using full-format
// message blocks with markdown enabled.
// Config: {"webhook_url": "https://integrations.zoom.us/chat/webhooks/incomingwebhook/...", "verification_token": "..."}
type Zoom struct{ HTTP *http.Client }

type zoomCfg struct {
	WebhookURL        string `json:"webhook_url"`
	VerificationToken string `json:"verification_token"`
}

func (z *Zoom) Send(ctx context.Context, raw json.RawMessage, m Message) error {
	var c zoomCfg
	if err := json.Unmarshal(raw, &c); err != nil || c.WebhookURL == "" {
		return errors.New("zoom: webhook_url required")
	}
	url := c.WebhookURL
	if !strings.Contains(url, "format=") {
		sep := "?"
		if strings.Contains(url, "?") {
			sep = "&"
		}
		url += sep + "format=full"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*Type:* %s\n*Source:* %s\n*Severity:* %s\n*Time:* %s\n*Rows captured:* %d",
		kindLabel(m.Kind), m.Source, strings.ToUpper(m.Severity), ts(m.Timestamp), m.RowCount)
	if m.Summary != "" {
		fmt.Fprintf(&b, "\n\n%s", m.Summary)
	}
	if m.Link != "" {
		fmt.Fprintf(&b, "\n\n<%s|Open incident →>", m.Link)
	}
	payload := map[string]any{
		"is_markdown_support": true,
		"content": map[string]any{
			"head": map[string]any{
				"text":     fmt.Sprintf("[%s] %s", label(m), m.Title),
				"sub_head": map[string]any{"text": "Sentinel Observability"},
			},
			"body": []any{map[string]any{"type": "message", "text": b.String(), "is_markdown_support": true}},
		},
	}
	h := map[string]string{}
	if c.VerificationToken != "" {
		h["Authorization"] = c.VerificationToken
	}
	return postJSON(ctx, z.HTTP, url, h, payload)
}
