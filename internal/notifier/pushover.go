package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Compile-time assertion that Pushover implements Provider
var _ Provider = (*Pushover)(nil)

type Pushover struct {
	AppToken string `json:"app_token"`
	UserKey  string `json:"user_key"`
	Priority int    `json:"priority"` // Optional (e.g., 1 for high, -1 for low)
}

func init() {
	Register("PUSHOVER", func(configJSON string) (Provider, error) {
		var p Pushover
		if err := json.Unmarshal([]byte(configJSON), &p); err != nil {
			return nil, fmt.Errorf("unmarshal pushover config: %w", err)
		}
		if p.AppToken == "" || p.UserKey == "" {
			return nil, fmt.Errorf("pushover config missing app_token or user_key")
		}
		return &p, nil
	})
}

func (p *Pushover) Type() string {
	return "PUSHOVER"
}

func (p *Pushover) Send(ctx context.Context, title, message string) error {
	data := url.Values{}
	data.Set("token", p.AppToken)
	data.Set("user", p.UserKey)
	data.Set("title", title)
	data.Set("message", message)
	
	if p.Priority != 0 {
		data.Set("priority", fmt.Sprintf("%d", p.Priority))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pushover.net/1/messages.json", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("pushover API returned status: %s", resp.Status)
	}

	return nil
}
