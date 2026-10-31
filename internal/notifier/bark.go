package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Compile-time assertion that Bark implements Provider
var _ Provider = (*Bark)(nil)

// Bark is a push notification provider for iOS via the Bark app
// (https://github.com/Finb/Bark). Defaults to the official server.
type Bark struct {
	Server string `json:"server"` // Optional, defaults to https://api.day.app
	Key    string `json:"key"`    // Device key from the Bark app

	// Optional push parameters
	Group string `json:"group"` // Notification grouping
	Sound string `json:"sound"` // Notification sound
	Icon  string `json:"icon"`  // Custom push icon URL
	URL   string `json:"url"`   // URL to open when the notification is tapped
	Level string `json:"level"` // Notification level: active, timeSensitive, passive
}

func init() {
	Register("bark", func(configJSON string) (Provider, error) {
		var b Bark
		if err := json.Unmarshal([]byte(configJSON), &b); err != nil {
			return nil, fmt.Errorf("unmarshal bark config: %w", err)
		}
		if b.Key == "" {
			return nil, fmt.Errorf("bark config missing key")
		}
		if b.Server == "" {
			b.Server = "https://api.day.app"
		}
		if b.Level != "" && b.Level != "active" && b.Level != "timeSensitive" && b.Level != "passive" {
			return nil, fmt.Errorf("bark config has invalid level %q (allowed: active, timeSensitive, passive)", b.Level)
		}
		return &b, nil
	})
}

func (b *Bark) Type() string {
	return "bark"
}

func (b *Bark) Send(ctx context.Context, endpointID int64, endpointName, title, message string) error {
	u := fmt.Sprintf("%s/%s/%s/%s",
		strings.TrimRight(b.Server, "/"),
		url.PathEscape(b.Key),
		url.PathEscape(title),
		url.PathEscape(message),
	)

	params := url.Values{}
	if b.Group != "" {
		params.Set("group", b.Group)
	}
	if b.Sound != "" {
		params.Set("sound", b.Sound)
	}
	if b.Icon != "" {
		params.Set("icon", b.Icon)
	}
	if b.URL != "" {
		params.Set("url", b.URL)
	}
	if b.Level != "" && b.Level != "active" {
		params.Set("level", b.Level)
	}
	if encoded := params.Encode(); encoded != "" {
		u += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bark API returned status: %s", resp.Status)
	}

	return nil
}