package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func postJSON(ctx context.Context, hc *http.Client, url string, headers map[string]string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return do(hc, req)
}

func do(hc *http.Client, req *http.Request) error {
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func label(m Message) string {
	if m.Event == "resolved" {
		return "RESOLVED"
	}
	return strings.ToUpper(m.Severity)
}

func kindLabel(k string) string {
	switch k {
	case "business_exception":
		return "Business Data Exception"
	case "runtime_exception":
		return "Runtime Application Exception"
	case "db_outage":
		return "Database Outage"
	case "endpoint_outage":
		return "Server / Endpoint Outage"
	}
	return k
}

func ts(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05 UTC") }
