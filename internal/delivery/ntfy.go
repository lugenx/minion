package delivery

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"minion/internal/types"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func SendNtfy(urlStr string, auth *BasicAuth, minionName string, item *types.Item) error {
	if urlStr == "" {
		return nil
	}

	req, err := http.NewRequest("POST", urlStr, bytes.NewBufferString(item.Summary))
	if err != nil {
		return err
	}

	if auth != nil && auth.Username != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	title := item.Title
	if len(title) > 64 {
		title = title[:61] + "..."
	}

	req.Header.Set("Title", title)
	req.Header.Set("Tags", minionName)

	if item.URL != "" {
		req.Header.Set("Click", item.URL)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send ntfy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status code %d", resp.StatusCode)
	}

	return nil
}
