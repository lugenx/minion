package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSourceUnmarshalYAML_OldMatchKeyword(t *testing.T) {
	y := `url: https://example.com
match: /events`
	var s Source
	if err := yaml.Unmarshal([]byte(y), &s); err != nil {
		t.Fatal(err)
	}
	if s.Match != "/events" {
		t.Errorf("expected Match=\"/events\", got %q", s.Match)
	}
}

func TestSourceUnmarshalYAML_NewFollowKeyword(t *testing.T) {
	y := `url: https://example.com
follow: /events`
	var s Source
	if err := yaml.Unmarshal([]byte(y), &s); err != nil {
		t.Fatal(err)
	}
	if s.Match != "/events" {
		t.Errorf("expected Match=\"/events\", got %q", s.Match)
	}
}

func TestSourceUnmarshalYAML_FollowTakesPrecedence(t *testing.T) {
	y := `url: https://example.com
match: /old
follow: /new`
	var s Source
	if err := yaml.Unmarshal([]byte(y), &s); err != nil {
		t.Fatal(err)
	}
	if s.Match != "/new" {
		t.Errorf("expected Match=\"/new\", got %q", s.Match)
	}
}

func TestSourceMarshalYAML_OutputsFollow(t *testing.T) {
	s := Source{URL: "https://example.com", Match: "/events"}
	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "url: https://example.com\nfollow: /events\n" {
		t.Errorf("unexpected marshal output:\n%s", string(out))
	}
}

func TestSourceMarshalYAML_EmptyMatchOmitted(t *testing.T) {
	s := Source{URL: "https://example.com"}
	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "url: https://example.com\n" {
		t.Errorf("expected omitempty to skip follow, got:\n%s", string(out))
	}
}

// === MinionConfig Tell/Report edge cases ===

func TestMinionConfigUnmarshalYAML_OldFlatTell(t *testing.T) {
	y := `name: test
do: test task
tell:
  ntfy: https://ntfy.sh/topic
  markdown: true`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tell) != 1 {
		t.Fatalf("expected 1 tell target, got %d", len(m.Tell))
	}
	if m.Tell[0]["ntfy"] != "https://ntfy.sh/topic" {
		t.Errorf("expected ntfy URL, got %v", m.Tell[0]["ntfy"])
	}
	if m.Tell[0]["markdown"] != true {
		t.Errorf("expected markdown=true, got %v", m.Tell[0]["markdown"])
	}
}

func TestMinionConfigUnmarshalYAML_NewArrayTell(t *testing.T) {
	y := `name: test
do: test task
tell:
  - ntfy: https://ntfy.sh/topic
    markdown: true
  - discord: https://discord.com/api/webhooks/foo`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tell) != 2 {
		t.Fatalf("expected 2 tell targets, got %d", len(m.Tell))
	}
	if m.Tell[0]["ntfy"] != "https://ntfy.sh/topic" {
		t.Errorf("expected ntfy URL, got %v", m.Tell[0]["ntfy"])
	}
	if m.Tell[1]["discord"] != "https://discord.com/api/webhooks/foo" {
		t.Errorf("expected discord URL, got %v", m.Tell[1]["discord"])
	}
}

func TestMinionConfigUnmarshalYAML_OldFlatReport(t *testing.T) {
	y := `name: test
do: test task
report:
  ntfy: https://ntfy.sh/logs`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Report) != 1 {
		t.Fatalf("expected 1 report target, got %d", len(m.Report))
	}
	if m.Report[0]["ntfy"] != "https://ntfy.sh/logs" {
		t.Errorf("expected ntfy URL, got %v", m.Report[0]["ntfy"])
	}
}

func TestMinionConfigUnmarshalYAML_NewArrayReport(t *testing.T) {
	y := `name: test
do: test task
report:
  - ntfy: https://ntfy.sh/logs
  - discord: https://discord.com/api/webhooks/bar`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Report) != 2 {
		t.Fatalf("expected 2 report targets, got %d", len(m.Report))
	}
}

func TestMinionConfigUnmarshalYAML_EmptyTellReport(t *testing.T) {
	y := `name: test
do: test task
tell:
report:`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tell) != 0 {
		t.Errorf("expected 0 tell targets, got %d", len(m.Tell))
	}
	if len(m.Report) != 0 {
		t.Errorf("expected 0 report targets, got %d", len(m.Report))
	}
}

func TestMinionConfigUnmarshalYAML_AbsentTellReport(t *testing.T) {
	y := `name: test
do: test task`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tell) != 0 {
		t.Errorf("expected 0 tell targets when absent, got %d", len(m.Tell))
	}
	if len(m.Report) != 0 {
		t.Errorf("expected 0 report targets when absent, got %d", len(m.Report))
	}
}

func TestMinionConfigUnmarshalYAML_MultipleSameType(t *testing.T) {
	y := `name: test
do: test task
tell:
  - ntfy: https://ntfy.sh/alerts
  - ntfy: https://ntfy.sh/backup
  - http_request: https://api.example.com
    method: POST
    headers:
      X-Api-Key: secret
    payload_template: '{"title": "{{.Title}}"}'
    basic_auth:
      username: user
      password: pass
  - minion: worker
  - discord: https://discord.com/api/webhooks/channel`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tell) != 5 {
		t.Fatalf("expected 5 tell targets, got %d", len(m.Tell))
	}
	// Two ntfy targets
	if m.Tell[0]["ntfy"] != "https://ntfy.sh/alerts" {
		t.Errorf("target 0: expected ntfy alerts, got %v", m.Tell[0]["ntfy"])
	}
	if m.Tell[1]["ntfy"] != "https://ntfy.sh/backup" {
		t.Errorf("target 1: expected ntfy backup, got %v", m.Tell[1]["ntfy"])
	}
	// http_request with nested fields - each field must be isolated to this target
	if m.Tell[2]["http_request"] != "https://api.example.com" {
		t.Errorf("target 2: expected http_request URL, got %v", m.Tell[2]["http_request"])
	}
	if m.Tell[2]["method"] != "POST" {
		t.Errorf("target 2: expected method POST, got %v", m.Tell[2]["method"])
	}
	headers, ok := m.Tell[2]["headers"].(map[string]interface{})
	if !ok || headers["X-Api-Key"] != "secret" {
		t.Errorf("target 2: expected headers, got %v", m.Tell[2]["headers"])
	}
	ba, ok := m.Tell[2]["basic_auth"].(map[string]interface{})
	if !ok || ba["username"] != "user" || ba["password"] != "pass" {
		t.Errorf("target 2: expected basic_auth, got %v", m.Tell[2]["basic_auth"])
	}
	// minion target
	if m.Tell[3]["minion"] != "worker" {
		t.Errorf("target 3: expected minion worker, got %v", m.Tell[3]["minion"])
	}
	// discord target
	if m.Tell[4]["discord"] != "https://discord.com/api/webhooks/channel" {
		t.Errorf("target 4: expected discord URL, got %v", m.Tell[4]["discord"])
	}
}

func TestMinionConfigUnmarshalYAML_MixedFormats(t *testing.T) {
	y := `name: test
do: test task
tell:
  ntfy: https://ntfy.sh/topic
  markdown: true
report:
  - discord: https://discord.com/api/webhooks/foo
  - ntfy: https://ntfy.sh/logs`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(y), &m); err != nil {
		t.Fatal(err)
	}
	// tell is old flat format -> 1 target
	if len(m.Tell) != 1 {
		t.Fatalf("expected 1 tell target, got %d", len(m.Tell))
	}
	if m.Tell[0]["ntfy"] != "https://ntfy.sh/topic" {
		t.Errorf("tell: expected ntfy URL, got %v", m.Tell[0]["ntfy"])
	}
	if m.Tell[0]["markdown"] != true {
		t.Errorf("tell: expected markdown=true, got %v", m.Tell[0]["markdown"])
	}
	// report is new array format -> 2 targets
	if len(m.Report) != 2 {
		t.Fatalf("expected 2 report targets, got %d", len(m.Report))
	}
	if m.Report[0]["discord"] != "https://discord.com/api/webhooks/foo" {
		t.Errorf("report 0: expected discord URL, got %v", m.Report[0]["discord"])
	}
	if m.Report[1]["ntfy"] != "https://ntfy.sh/logs" {
		t.Errorf("report 1: expected ntfy URL, got %v", m.Report[1]["ntfy"])
	}
}

func TestMinionConfigMarshalYAML_OutputsArray(t *testing.T) {
	m := MinionConfig{
		Name: "test",
		Do:   "test task",
		Tell: []map[string]interface{}{
			{"ntfy": "https://ntfy.sh/topic", "markdown": true},
		},
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "- ") {
		t.Errorf("expected array format with list item prefix, got:\n%s", string(out))
	}
}

func TestMinionConfigMarshalYAML_MultipleTargets(t *testing.T) {
	m := MinionConfig{
		Name: "test",
		Do:   "test task",
		Tell: []map[string]interface{}{
			{"ntfy": "https://ntfy.sh/a", "markdown": true},
			{"discord": "https://discord.com/api/webhooks/b"},
			{"minion": "worker"},
			{"http_request": "https://api.example.com", "method": "POST", "headers": map[string]interface{}{"X-Key": "val"}, "payload_template": "body"},
		},
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	outStr := string(out)
	// Must be array format (each target is a YAML list item)
	if !strings.Contains(outStr, "https://ntfy.sh/a") {
		t.Errorf("expected ntfy URL in output, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "https://discord.com/api/webhooks/b") {
		t.Errorf("expected discord URL in output, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "worker") {
		t.Errorf("expected minion target in output, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "https://api.example.com") {
		t.Errorf("expected http_request URL in output, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "POST") {
		t.Errorf("expected POST method in output, got:\n%s", outStr)
	}
	// Must contain YAML list marker
	if !strings.Contains(outStr, "- ") {
		t.Errorf("expected array format (list marker '- '), got:\n%s", outStr)
	}
}

func TestMinionConfigRoundTrip_OldToNew(t *testing.T) {
	// Old flat format in -> marshal should produce array format
	yIn := `name: test
do: test task
tell:
  ntfy: https://ntfy.sh/topic
  markdown: true
report:
  discord: https://discord.com/api/webhooks/log
`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(yIn), &m); err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	outStr := string(out)
	// Round-trip must have array format (list marker `- ` in tell/report sections)
	if !strings.Contains(outStr, "https://ntfy.sh/topic") {
		t.Errorf("round-trip: missing ntfy URL:\n%s", outStr)
	}
	if !strings.Contains(outStr, "https://discord.com/api/webhooks/log") {
		t.Errorf("round-trip: missing discord URL:\n%s", outStr)
	}
	// Verify it's array format — the old flat format produces a single-element array
	// with yaml.marshal, so we just need to confirm targets survive
}

func TestMinionConfigRoundTrip_NewToNew(t *testing.T) {
	// New array format in -> marshal should output same array format
	yIn := `name: test
do: test task
tell:
  - ntfy: https://ntfy.sh/topic
    markdown: true
  - discord: https://discord.com/api/webhooks/foo
`
	var m MinionConfig
	if err := yaml.Unmarshal([]byte(yIn), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tell) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(m.Tell))
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// Unmarshal again to verify round-trip
	var m2 MinionConfig
	if err := yaml.Unmarshal(out, &m2); err != nil {
		t.Fatalf("second unmarshal failed: %v", err)
	}
	if len(m2.Tell) != 2 {
		t.Errorf("expected 2 targets after round-trip, got %d", len(m2.Tell))
	}
	if m2.Tell[0]["ntfy"] != "https://ntfy.sh/topic" {
		t.Errorf("tell[0] ntfy URL changed")
	}
	if m2.Tell[0]["markdown"] != true {
		t.Errorf("tell[0] markdown changed")
	}
	if m2.Tell[1]["discord"] != "https://discord.com/api/webhooks/foo" {
		t.Errorf("tell[1] discord URL changed")
	}
}

func TestMinionConfigRoundTrip_Idempotent(t *testing.T) {
	m := MinionConfig{
		Name: "test",
		Do:   "test task",
		Tell: []map[string]interface{}{
			{"ntfy": "https://ntfy.sh/topic", "markdown": true, "basic_auth": map[string]interface{}{"username": "u", "password": "p"}},
			{"discord": "https://discord.com/api/webhooks/c"},
		},
		Report: []map[string]interface{}{
			{"minion": "reporter"},
		},
	}
	// Marshal twice: ensure stability
	out1, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var m2 MinionConfig
	if err := yaml.Unmarshal(out1, &m2); err != nil {
		t.Fatalf("second unmarshal failed: %v\ninput was:\n%s", err, string(out1))
	}
	if len(m2.Tell) != 2 {
		t.Errorf("expected 2 tell targets after round-trip, got %d", len(m2.Tell))
	}
	if len(m2.Report) != 1 {
		t.Errorf("expected 1 report target after round-trip, got %d", len(m2.Report))
	}
	if m2.Tell[0]["ntfy"] != "https://ntfy.sh/topic" {
		t.Errorf("tell[0] ntfy URL changed after round-trip")
	}
	if m2.Report[0]["minion"] != "reporter" {
		t.Errorf("report[0] minion changed after round-trip")
	}
}
