//go:build !windows

package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"minion/internal/config"
	"minion/internal/types"
)

func TestProcessCommandItem_TimeoutTerminatesDescendantsPromptly(t *testing.T) {
	runCtx := &RunContext{Stats: &Stats{}, Ephemeral: true}
	m := &config.MinionConfig{
		Name:     "timeout-test",
		Filename: "timeout-test.yaml",
		Settings: config.Settings{Timeout: 1},
	}
	item := &types.Item{SourceType: "command", Command: "sleep 3; printf late"}

	started := time.Now()
	if err := processCommandItem(context.Background(), m, item, runCtx); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)

	if elapsed >= 2*time.Second {
		t.Fatalf("1s timeout waited for shell descendant: %s", elapsed)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected one timeout error, got %d", runCtx.Stats.Errors)
	}
}

func TestProcessCommandItem_ContextCancellationTerminatesDescendantsPromptly(t *testing.T) {
	runCtx := &RunContext{Stats: &Stats{}, Ephemeral: true}
	m := &config.MinionConfig{Name: "cancel-test", Filename: "cancel-test.yaml"}
	item := &types.Item{SourceType: "command", Command: "sleep 3; printf late"}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)

	started := time.Now()
	err := processCommandItem(ctx, m, item, runCtx)
	elapsed := time.Since(started)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if elapsed >= 1500*time.Millisecond {
		t.Fatalf("context cancellation waited for shell descendant: %s", elapsed)
	}
}

func TestProcessCommandItem_TimeoutClosesPipesFromBackgroundDescendant(t *testing.T) {
	runCtx := &RunContext{Stats: &Stats{}, Ephemeral: true}
	m := &config.MinionConfig{
		Name:     "background-timeout-test",
		Filename: "background-timeout-test.yaml",
		Settings: config.Settings{Timeout: 1},
	}
	item := &types.Item{SourceType: "command", Command: "sleep 3 &"}

	started := time.Now()
	if err := processCommandItem(context.Background(), m, item, runCtx); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)

	if elapsed >= 2*time.Second {
		t.Fatalf("timeout waited for background descendant pipe: %s", elapsed)
	}
	if runCtx.Stats.Errors != 1 {
		t.Fatalf("expected one timeout error, got %d", runCtx.Stats.Errors)
	}
}

func TestProcessCommandItem_TimeoutKillsBackgroundDescendant(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "survived")
	runCtx := &RunContext{Stats: &Stats{}, Ephemeral: true}
	m := &config.MinionConfig{
		Name:     "background-kill-test",
		Filename: "background-kill-test.yaml",
		Settings: config.Settings{Timeout: 1},
	}
	command := fmt.Sprintf("(sleep 2; printf survived > %q) &", marker)
	item := &types.Item{SourceType: "command", Command: command}

	if err := processCommandItem(context.Background(), m, item, runCtx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("background descendant survived timeout and wrote %s", marker)
	}
}
