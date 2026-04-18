package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Compile-time assertion that Telegram implements Provider
var _ Provider = (*Telegram)(nil)

type Telegram struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

func init() {
	Register("TELEGRAM", func(configJSON string) (Provider, error) {
		var t Telegram
		if err := json.Unmarshal([]byte(configJSON), &t); err != nil {
			return nil, fmt.Errorf("unmarshal telegram config: %w", err)
		}
		if t.BotToken == "" || t.ChatID == "" {
			return nil, fmt.Errorf("telegram config missing bot_token or chat_id")
		}
		return &t, nil
	})
}

func (t *Telegram) Type() string {
	return "TELEGRAM"
}

func (t *Telegram) Send(ctx context.Context, title, message string) error {
	// Combine title and message for Telegram (which doesn't have a native title field)
	fullText := fmt.Sprintf("<b>%s</b>\n\n%s", title, message)

	payload := map[string]interface{}{
		"chat_id":    t.ChatID,
		"text":       fullText,
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API returned status: %s", resp.Status)
	}

	return nil
}
