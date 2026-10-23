package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type EmailSettings struct {
	Host, User, Pass, From string
	Port                   int
}

// Email sends a mobile-responsive HTML email via SMTP (STARTTLS when offered).
// Config: {"to": ["oncall@company.com", "dba@company.com"]}
type Email struct{ Settings EmailSettings }

type emailCfg struct {
	To []string `json:"to"`
}

func (e *Email) Send(ctx context.Context, raw json.RawMessage, m Message) error {
	var c emailCfg
	if err := json.Unmarshal(raw, &c); err != nil || len(c.To) == 0 {
		return errors.New("email: to[] required")
	}
	if e.Settings.Host == "" {
		return errors.New("email: SMTP_HOST not configured")
	}
	var html bytes.Buffer
	if err := emailTpl.Execute(&html, map[string]any{
		"M": m, "Label": label(m), "Kind": kindLabel(m.Kind), "Time": ts(m.Timestamp),
		"Color":    map[string]string{"critical": "#dc2626", "warning": "#d97706", "info": "#2563eb"}[m.Severity],
		"Resolved": m.Event == "resolved",
	}); err != nil {
		return err
	}
	subject := fmt.Sprintf("[%s] %s — %s", label(m), m.Title, m.Source)
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		e.Settings.From, strings.Join(c.To, ", "), mime.QEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z))
	msg.Write(html.Bytes())

	addr := net.JoinHostPort(e.Settings.Host, strconv.Itoa(e.Settings.Port))
	var auth smtp.Auth
	if e.Settings.User != "" {
		auth = smtp.PlainAuth("", e.Settings.User, e.Settings.Pass, e.Settings.Host)
	}
	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(addr, auth, e.Settings.From, c.To, msg.Bytes()) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

var emailTpl = template.Must(template.New("email").Parse(`<!doctype html>
<html><head><meta name="viewport" content="width=device-width,initial-scale=1"><meta charset="utf-8">
<style>
 @media (max-width:600px){ .card{width:100%!important;border-radius:0!important} .pad{padding:20px!important} }
</style></head>
<body style="margin:0;background:#f1f5f9;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;color:#0f172a">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f1f5f9;padding:24px 0">
<tr><td align="center">
 <table role="presentation" class="card" width="560" cellpadding="0" cellspacing="0" style="background:#fff;border-radius:12px;overflow:hidden;border:1px solid #e2e8f0">
  <tr><td style="height:6px;background:{{if .Resolved}}#16a34a{{else}}{{.Color}}{{end}}"></td></tr>
  <tr><td class="pad" style="padding:28px 32px">
   <div style="font-size:12px;letter-spacing:.08em;font-weight:700;color:{{if .Resolved}}#16a34a{{else}}{{.Color}}{{end}}">{{.Label}} · {{.Kind}}</div>
   <h1 style="margin:8px 0 16px;font-size:22px;line-height:1.3">{{.M.Title}}</h1>
   <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="font-size:14px;line-height:1.6">
    <tr><td style="color:#64748b;width:130px">Source</td><td style="font-weight:600">{{.M.Source}}</td></tr>
    <tr><td style="color:#64748b">Timestamp</td><td>{{.Time}}</td></tr>
    <tr><td style="color:#64748b">Rows captured</td><td>{{.M.RowCount}}</td></tr>
   </table>
   {{if .M.Summary}}<p style="margin:16px 0 0;font-size:14px;color:#334155">{{.M.Summary}}</p>{{end}}
   {{if .M.Link}}
   <table role="presentation" cellpadding="0" cellspacing="0" style="margin-top:24px"><tr><td style="border-radius:8px;background:#0f172a">
    <a href="{{.M.Link}}" style="display:inline-block;padding:12px 22px;color:#fff;text-decoration:none;font-weight:600;font-size:14px">Open incident →</a>
   </td></tr></table>
   <p style="margin:16px 0 0;font-size:12px;color:#94a3b8">Captured data is not included in this email for compliance. Sign in to view it.</p>
   {{end}}
  </td></tr>
 </table>
</td></tr></table></body></html>`))
