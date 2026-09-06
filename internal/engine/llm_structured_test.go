package engine

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

type structuredRetryTestResult struct {
	Matches []struct {
		Title string `json:"title"`
	} `json:"matches"`
}

func TestRequestStructured_RetriesOneSchemaFailure(t *testing.T) {
	var systemPrompts, userPrompts []string
	calls := 0
	chat := func(_ context.Context, model, systemPrompt, userPrompt string, jsonMode bool) (string, float64, error) {
		calls++
		if model != "test-model" || !jsonMode {
			t.Fatalf("unexpected chat arguments: model=%q json=%t", model, jsonMode)
		}
		systemPrompts = append(systemPrompts, systemPrompt)
		userPrompts = append(userPrompts, userPrompt)
		if calls == 1 {
			return `{"matches":[123]}`, 0.1, nil
		}
		return "```json\n{\"matches\":[{\"title\":\"repaired\"}]}\n```", 0.2, nil
	}

	got, cost, err := requestStructured[structuredRetryTestResult](
		context.Background(), "test-model", "system task", "record data", chat,
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("got %d calls, want 2", calls)
	}
	if len(got.Matches) != 1 || got.Matches[0].Title != "repaired" {
		t.Fatalf("unexpected repaired result: %#v", got)
	}
	if math.Abs(cost-0.3) > 1e-9 {
		t.Fatalf("cost = %f, want 0.3", cost)
	}
	if userPrompts[0] != "record data" || userPrompts[1] != userPrompts[0] {
		t.Fatalf("retry changed record data: %#v", userPrompts)
	}
	if systemPrompts[0] != "system task" || !strings.HasPrefix(systemPrompts[1], "system task") {
		t.Fatalf("retry changed the original task: %#v", systemPrompts)
	}
	if !strings.Contains(systemPrompts[1], "cannot unmarshal") {
		t.Fatalf("retry prompt lacks parse correction context: %q", systemPrompts[1])
	}
}

func TestRequestStructured_StopsAfterOneRepairRetry(t *testing.T) {
	calls := 0
	chat := func(context.Context, string, string, string, bool) (string, float64, error) {
		calls++
		return `{"matches":[123]}`, 0.25, nil
	}

	_, cost, err := requestStructured[structuredRetryTestResult](
		context.Background(), "test-model", "system task", "record data", chat,
	)
	if err == nil {
		t.Fatal("expected schema error after bounded retry")
	}
	if calls != 2 {
		t.Fatalf("got %d calls, want exactly 2", calls)
	}
	if math.Abs(cost-0.5) > 1e-9 {
		t.Fatalf("cost = %f, want 0.5", cost)
	}
}

func TestRequestStructured_DoesNotRepairTransportFailure(t *testing.T) {
	calls := 0
	transportErr := errors.New("transport failed")
	chat := func(context.Context, string, string, string, bool) (string, float64, error) {
		calls++
		return "", 0, transportErr
	}

	_, _, err := requestStructured[structuredRetryTestResult](
		context.Background(), "test-model", "system task", "record data", chat,
	)
	if !errors.Is(err, transportErr) {
		t.Fatalf("expected transport error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("transport failure was retried by schema repair: %d calls", calls)
	}
}
