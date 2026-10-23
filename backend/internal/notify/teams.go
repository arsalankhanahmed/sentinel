package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// Teams posts an Adaptive Card (v1.4) to a Teams Workflows / incoming-webhook URL.
// Config: {"webhook_url": "https://...logic.azure.com/..."}
type Teams struct{ HTTP *http.Client }

type teamsCfg struct {
	WebhookURL string `json:"webhook_url"`
}

func (t *Teams) Send(ctx context.Context, raw json.RawMessage, m Message) error {
	var c teamsCfg
	if err := json.Unmarshal(raw, &c); err != nil || c.WebhookURL == "" {
		return errors.New("teams: webhook_url required")
	}
	color := map[string]string{"critical": "Attention", "warning": "Warning", "info": "Accent"}[m.Severity]
	if m.Event == "resolved" {
		color = "Good"
	}
	body := []any{
		map[string]any{"type": "TextBlock", "text": fmt.Sprintf("%s · %s", label(m), kindLabel(m.Kind)),
			"weight": "Bolder", "color": color, "size": "Small", "spacing": "None"},
		map[string]any{"type": "TextBlock", "text": m.Title, "weight": "Bolder", "size": "Large", "wrap": true},
		map[string]any{"type": "FactSet", "facts": []map[string]string{
			{"title": "Exception", "value": m.Title},
			{"title": "Source", "value": m.Source},
			{"title": "Timestamp", "value": ts(m.Timestamp)},
			{"title": "Rows captured", "value": fmt.Sprint(m.RowCount)},
		}},
	}
	if m.Summary != "" {
		body = append(body, map[string]any{"type": "TextBlock", "text": m.Summary, "wrap": true, "isSubtle": true})
	}
	card := map[string]any{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard", "version": "1.4",
		"msteams": map[string]any{"width": "Full"},
		"body":    body,
	}
	if m.Link != "" {
		card["actions"] = []any{map[string]any{"type": "Action.OpenUrl", "title": "Open Incident", "url": m.Link, "style": "positive"}}
	}
	return postJSON(ctx, t.HTTP, c.WebhookURL, nil, map[string]any{
		"type": "message",
		"attachments": []any{map[string]any{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"content":     card,
		}},
	})
}
