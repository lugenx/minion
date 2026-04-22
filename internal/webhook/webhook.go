package webhook

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"minion/internal/config"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// Send POSTs a raw text message to the configured webhook URL.
func Send(webhookConfig *config.Webhook, title, message, clickURL string) error {
	if webhookConfig == nil || webhookConfig.URL == "" {
		return nil
	}

	method := webhookConfig.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhookConfig.URL, bytes.NewBufferString(message))
	if err != nil {
		return err
	}

	// Apply generic Basic Auth if configured
	if webhookConfig.BasicAuth != nil && webhookConfig.BasicAuth.Username != "" {
		req.SetBasicAuth(webhookConfig.BasicAuth.Username, webhookConfig.BasicAuth.Password)
	}

	// Apply custom headers from YAML
	if webhookConfig.Headers != nil {
		for k, v := range webhookConfig.Headers {
			req.Header.Set(k, v)
		}
	}

	// Always inject the default Title and Click URL headers (which ntfy reads)
	// But don't overwrite them if the user manually specified them in YAML
	if req.Header.Get("Title") == "" {
		req.Header.Set("Title", title)
	}
	if clickURL != "" && req.Header.Get("Click") == "" {
		req.Header.Set("Click", clickURL)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status code %d", resp.StatusCode)
	}

	return nil
}