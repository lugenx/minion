package delivery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"minion/internal/types"
)

type DiscordPayload struct {
	Content string `json:"content"`
}

func SendDiscord(urlStr string, minionName string, item *types.Item) error {
	if urlStr == "" {
		return nil
	}

	msg := fmt.Sprintf("**%s**\n%s", item.Title, item.Summary)
	if item.URL != "" {
		msg += fmt.Sprintf("\n[Link](%s)", item.URL)
	}

	payload := DiscordPayload{
		Content: msg,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status code %d", resp.StatusCode)
	}

	return nil
}
