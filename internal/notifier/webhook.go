package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"text/template"
)

// Compile-time assertion that Webhook implements Provider
var _ Provider = (*Webhook)(nil)

// Webhook is a generic outgoing webhook provider. By default it POSTs a JSON
// body with the alert details. When Body is set, it is rendered as a template
// supporting {{.Title}}, {{.Message}}, {{.Endpoint}} and {{.EndpointID}}.
type Webhook struct {
	URL         string            `json:"url"`          // Target URL (required)
	Method      string            `json:"method"`       // post (default), put, patch, get
	ContentType string            `json:"content_type"` // json (default), form, text
	Headers     map[string]string `json:"headers"`      // Custom headers (e.g. Authorization)
	Body        string            `json:"body"`         // Optional body template
}

type webhookTemplateData struct {
	Title      string
	Message    string
	Endpoint   string
	EndpointID int64
}

var webhookContentTypes = map[string]string{
	"json": "application/json",
	"form": "application/x-www-form-urlencoded",
	"text": "text/plain",
}

func init() {
	Register("webhook", func(configJSON string) (Provider, error) {
		var w Webhook
		if err := json.Unmarshal([]byte(configJSON), &w); err != nil {
			return nil, fmt.Errorf("unmarshal webhook config: %w", err)
		}

		if w.URL == "" {
			return nil, fmt.Errorf("webhook config missing url")
		}
		u, err := url.Parse(w.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("webhook config url must be a valid http or https URL")
		}

		if w.Method == "" {
			w.Method = "post"
		}
		w.Method = strings.ToLower(w.Method)
		switch w.Method {
		case "post", "put", "patch", "get":
		default:
			return nil, fmt.Errorf("webhook config has invalid method %q (allowed: post, put, patch, get)", w.Method)
		}

		if w.ContentType == "" {
			w.ContentType = "json"
		}
		if _, ok := webhookContentTypes[w.ContentType]; !ok {
			return nil, fmt.Errorf("webhook config has invalid content_type %q (allowed: json, form, text)", w.ContentType)
		}

		// The default JSON body only makes sense for JSON. Other formats need a template.
		if w.ContentType != "json" && w.Body == "" {
			return nil, fmt.Errorf("webhook config requires a body template for content_type %q", w.ContentType)
		}

		return &w, nil
	})
}

func (w *Webhook) Type() string {
	return "webhook"
}

func (w *Webhook) Send(ctx context.Context, endpointID int64, endpointName, title, message string) error {
	body, err := w.buildBody(endpointID, endpointName, title, message)
	if err != nil {
		return fmt.Errorf("webhook build body: %w", err)
	}

	method := strings.ToUpper(w.Method)
	var reader io.Reader
	if method != http.MethodGet && len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, w.URL, reader)
	if err != nil {
		return err
	}

	if method != http.MethodGet {
		req.Header.Set("Content-Type", webhookContentTypes[w.ContentType])
	}
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status: %s", resp.Status)
	}

	return nil
}

// buildBody renders the configured template, or falls back to default JSON.
func (w *Webhook) buildBody(endpointID int64, endpointName, title, message string) ([]byte, error) {
	if w.Body != "" {
		tpl, err := template.New("body").Parse(w.Body)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, webhookTemplateData{
			Title:      title,
			Message:    message,
			Endpoint:   endpointName,
			EndpointID: endpointID,
		}); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	return json.Marshal(map[string]interface{}{
		"title":       title,
		"message":     message,
		"endpoint":    endpointName,
		"endpoint_id": endpointID,
	})
}
