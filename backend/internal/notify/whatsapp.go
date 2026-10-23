package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// WhatsApp supports Meta Cloud API (approved template) and Twilio.
//
// Meta config:
//
//	{"provider":"meta","phone_number_id":"...","access_token":"...","template":"incident_alert",
//	 "language":"en","recipients":["9230012345678"],"api_version":"v21.0"}
//	Template body params: {{1}} severity, {{2}} title, {{3}} source, {{4}} time, {{5}} link
//
// Twilio config:
//
//	{"provider":"twilio","account_sid":"AC...","auth_token":"...","from":"whatsapp:+14155238886",
//	 "recipients":["whatsapp:+923001234567"],"content_sid":""}
type WhatsApp struct{ HTTP *http.Client }

type waCfg struct {
	Provider      string   `json:"provider"`
	Recipients    []string `json:"recipients"`
	PhoneNumberID string   `json:"phone_number_id"`
	AccessToken   string   `json:"access_token"`
	Template      string   `json:"template"`
	Language      string   `json:"language"`
	APIVersion    string   `json:"api_version"`
	AccountSID    string   `json:"account_sid"`
	AuthToken     string   `json:"auth_token"`
	From          string   `json:"from"`
	ContentSID    string   `json:"content_sid"`
	// BaseURL overrides the provider API host (egress proxy, sandbox, or mock receiver).
	BaseURL string `json:"base_url"`
}

func (w *WhatsApp) Send(ctx context.Context, raw json.RawMessage, m Message) error {
	var c waCfg
	if err := json.Unmarshal(raw, &c); err != nil || len(c.Recipients) == 0 {
		return errors.New("whatsapp: recipients required")
	}
	link := m.Link
	if link == "" {
		link = "-"
	}
	var errs []string
	for _, to := range c.Recipients {
		var err error
		switch c.Provider {
		case "twilio":
			err = w.twilio(ctx, c, to, m, link)
		default:
			err = w.meta(ctx, c, to, m, link)
		}
		if err != nil {
			errs = append(errs, to+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (w *WhatsApp) meta(ctx context.Context, c waCfg, to string, m Message, link string) error {
	ver := c.APIVersion
	if ver == "" {
		ver = "v21.0"
	}
	lang := c.Language
	if lang == "" {
		lang = "en"
	}
	param := func(s string) map[string]string { return map[string]string{"type": "text", "text": s} }
	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template": map[string]any{
			"name":     c.Template,
			"language": map[string]string{"code": lang},
			"components": []any{map[string]any{
				"type":       "body",
				"parameters": []any{param(label(m)), param(m.Title), param(m.Source), param(ts(m.Timestamp)), param(link)},
			}},
		},
	}
	host := "https://graph.facebook.com"
	if c.BaseURL != "" {
		host = strings.TrimRight(c.BaseURL, "/")
	}
	u := fmt.Sprintf("%s/%s/%s/messages", host, ver, c.PhoneNumberID)
	return postJSON(ctx, w.HTTP, u, map[string]string{"Authorization": "Bearer " + c.AccessToken}, body)
}

func (w *WhatsApp) twilio(ctx context.Context, c waCfg, to string, m Message, link string) error {
	form := url.Values{"To": {to}, "From": {c.From}}
	if c.ContentSID != "" {
		vars, _ := json.Marshal(map[string]string{"1": label(m), "2": m.Title, "3": m.Source, "4": ts(m.Timestamp), "5": link})
		form.Set("ContentSid", c.ContentSID)
		form.Set("ContentVariables", string(vars))
	} else {
		form.Set("Body", fmt.Sprintf("🚨 *%s* — %s\nSource: %s\nTime: %s\nDetails: %s",
			label(m), m.Title, m.Source, ts(m.Timestamp), link))
	}
	host := "https://api.twilio.com"
	if c.BaseURL != "" {
		host = strings.TrimRight(c.BaseURL, "/")
	}
	u := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Messages.json", host, c.AccountSID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.AccountSID, c.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(w.HTTP, req)
}
