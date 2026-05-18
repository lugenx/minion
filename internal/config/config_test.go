package config

import (
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
