package delivery

import (
	"bytes"
	"fmt"
	"net/http"
	"text/template"

	"minion/internal/types"
)

type HTTPRequestConfig struct {
	URL             string            `yaml:"url"`
	Method          string            `yaml:"method"`
	BasicAuth       *BasicAuth        `yaml:"basic_auth"`
	Headers         map[string]string `yaml:"headers"`
	PayloadTemplate string            `yaml:"payload_template"`
}

func SendHTTPRequest(config *HTTPRequestConfig, item *types.Item) error {
	if config == nil || config.URL == "" {
		return nil
	}

	method := config.Method
	if method == "" {
		method = "POST"
	}

	var body []byte
	if config.PayloadTemplate != "" {
		tmpl, err := template.New("payload").Parse(config.PayloadTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse payload template: %w", err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, item); err != nil {
			return fmt.Errorf("failed to execute payload template: %w", err)
		}
		body = buf.Bytes()
	}

	req, err := http.NewRequest(method, config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	if config.BasicAuth != nil && config.BasicAuth.Username != "" {
		req.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	}

	if config.Headers != nil {
		for k, v := range config.Headers {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http request returned status code %d", resp.StatusCode)
	}

	return nil
}
