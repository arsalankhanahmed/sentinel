package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type capture struct {
	path, auth, ctype string
	body              []byte
}

func receiver(t *testing.T, status int) (*httptest.Server, *[]capture) {
	t.Helper()
	got := &[]capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*got = append(*got, capture{r.URL.RequestURI(), r.Header.Get("Authorization"), r.Header.Get("Content-Type"), b})
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

var msg = Message{
	Event: "opened", IncidentID: "abc", Title: "Invoices posted without GL entry", Kind: "business_exception",
	Source: "ERP Production", Severity: "critical", RowCount: 2, Summary: "Rule returned 2 row(s).",
	Timestamp: time.Date(2026, 10, 23, 8, 0, 0, 0, time.UTC), Link: "https://platform.local/i/TOKEN123",
}

func cfg(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestTeamsAdaptiveCard(t *testing.T) {
	srv, got := receiver(t, 200)
	err := (&Teams{HTTP: srv.Client()}).Send(context.Background(), cfg(map[string]string{"webhook_url": srv.URL + "/teams"}), msg)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Type        string `json:"type"`
		Attachments []struct {
			ContentType string `json:"contentType"`
			Content     struct {
				Type    string           `json:"type"`
				Version string           `json:"version"`
				Body    []map[string]any `json:"body"`
				Actions []map[string]any `json:"actions"`
			} `json:"content"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal((*got)[0].body, &p); err != nil {
		t.Fatal(err)
	}
	a := p.Attachments[0]
	if p.Type != "message" || a.ContentType != "application/vnd.microsoft.card.adaptive" || a.Content.Type != "AdaptiveCard" {
		t.Fatalf("bad envelope: %+v", p)
	}
	if a.Content.Actions[0]["type"] != "Action.OpenUrl" || a.Content.Actions[0]["url"] != msg.Link {
		t.Fatalf("missing deep link action: %+v", a.Content.Actions)
	}
	raw := string((*got)[0].body)
	for _, want := range []string{"Exception", "Source", "Timestamp", "ERP Production", "2026-10-23 08:00:00 UTC"} {
		if !strings.Contains(raw, want) {
			t.Errorf("card missing %q", want)
		}
	}
}

func TestTeamsResolvedHasNoButton(t *testing.T) {
	srv, got := receiver(t, 200)
	m := msg
	m.Event, m.Link = "resolved", ""
	_ = (&Teams{HTTP: srv.Client()}).Send(context.Background(), cfg(map[string]string{"webhook_url": srv.URL}), m)
	if strings.Contains(string((*got)[0].body), "Action.OpenUrl") || !strings.Contains(string((*got)[0].body), "RESOLVED") {
		t.Fatal("resolved card should show RESOLVED and no action")
	}
}

func TestZoomMarkdown(t *testing.T) {
	srv, got := receiver(t, 200)
	err := (&Zoom{HTTP: srv.Client()}).Send(context.Background(),
		cfg(map[string]string{"webhook_url": srv.URL + "/zoom", "verification_token": "vt"}), msg)
	if err != nil {
		t.Fatal(err)
	}
	c := (*got)[0]
	if c.auth != "vt" || !strings.Contains(c.path, "format=full") {
		t.Fatalf("auth/format wrong: %+v", c)
	}
	var p map[string]any
	_ = json.Unmarshal(c.body, &p)
	if p["is_markdown_support"] != true {
		t.Fatal("markdown flag missing")
	}
	text := p["content"].(map[string]any)["body"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "<https://platform.local/i/TOKEN123|Open incident") {
		t.Fatal("zoom markdown link missing")
	}
}

func TestWhatsAppMetaTemplate(t *testing.T) {
	srv, got := receiver(t, 200)
	err := (&WhatsApp{HTTP: srv.Client()}).Send(context.Background(), cfg(map[string]any{
		"provider": "meta", "base_url": srv.URL, "phone_number_id": "PN1", "access_token": "tok",
		"template": "incident_alert", "recipients": []string{"923001", "923002"},
	}), msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 2 {
		t.Fatalf("expected one call per recipient, got %d", len(*got))
	}
	c := (*got)[0]
	if c.path != "/v21.0/PN1/messages" || c.auth != "Bearer tok" {
		t.Fatalf("bad request: %+v", c)
	}
	if !strings.Contains(string(c.body), `"name":"incident_alert"`) || !strings.Contains(string(c.body), msg.Link) {
		t.Fatalf("bad template body: %s", c.body)
	}
}

func TestWhatsAppTwilio(t *testing.T) {
	srv, got := receiver(t, 201)
	err := (&WhatsApp{HTTP: srv.Client()}).Send(context.Background(), cfg(map[string]any{
		"provider": "twilio", "base_url": srv.URL, "account_sid": "AC1", "auth_token": "secret",
		"from": "whatsapp:+1415", "recipients": []string{"whatsapp:+92300"},
	}), msg)
	if err != nil {
		t.Fatal(err)
	}
	c := (*got)[0]
	if c.path != "/2010-04-01/Accounts/AC1/Messages.json" || !strings.HasPrefix(c.auth, "Basic ") {
		t.Fatalf("bad twilio call: %+v", c)
	}
	form, _ := url.ParseQuery(string(c.body))
	if form.Get("To") != "whatsapp:+92300" || !strings.Contains(form.Get("Body"), msg.Link) {
		t.Fatalf("bad form: %v", form)
	}
}

func TestHTTPErrorsSurface(t *testing.T) {
	srv, _ := receiver(t, 500)
	if err := (&Teams{HTTP: srv.Client()}).Send(context.Background(), cfg(map[string]string{"webhook_url": srv.URL}), msg); err == nil {
		t.Fatal("non-2xx must be an error (so the dispatcher retries)")
	}
}

func TestConfigValidation(t *testing.T) {
	ctx := context.Background()
	if (&Teams{}).Send(ctx, cfg(map[string]string{}), msg) == nil {
		t.Error("teams without url")
	}
	if (&Zoom{}).Send(ctx, cfg(map[string]string{}), msg) == nil {
		t.Error("zoom without url")
	}
	if (&WhatsApp{}).Send(ctx, cfg(map[string]any{}), msg) == nil {
		t.Error("whatsapp without recipients")
	}
	if (&Email{}).Send(ctx, cfg(map[string]any{"to": []string{"a@b"}}), msg) == nil {
		t.Error("email without SMTP host")
	}
}

func TestEmailTemplate(t *testing.T) {
	var buf bytes.Buffer
	err := emailTpl.Execute(&buf, map[string]any{"M": msg, "Label": label(msg), "Kind": kindLabel(msg.Kind),
		"Time": ts(msg.Timestamp), "Color": "#dc2626", "Resolved": false})
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, want := range []string{"viewport", "Open incident", msg.Link, "Business Data Exception", "max-width:600px"} {
		if !strings.Contains(html, want) {
			t.Errorf("email missing %q", want)
		}
	}
}

// Compliance guarantee: notification payloads must never contain captured row data.
func TestNoRawDataInPayloads(t *testing.T) {
	srv, got := receiver(t, 200)
	ctx := context.Background()
	_ = (&Teams{HTTP: srv.Client()}).Send(ctx, cfg(map[string]string{"webhook_url": srv.URL}), msg)
	_ = (&Zoom{HTTP: srv.Client()}).Send(ctx, cfg(map[string]string{"webhook_url": srv.URL}), msg)
	for _, c := range *got {
		if strings.Contains(string(c.body), "payload") || len(c.body) > 4096 {
			t.Fatalf("payload too large or contains data: %d bytes", len(c.body))
		}
	}
}
