package store

import (
	"os"
	"testing"
)

func TestStore(t *testing.T) {
	dbPath := "/tmp/minion_test_" + t.Name() + ".db"
	defer os.Remove(dbPath)

	s, err := InitStore(dbPath)
	if err != nil {
		t.Fatalf("InitStore failed: %v", err)
	}
	defer s.Close()

	// Page hash
	h, err := s.GetPageHash("http://example.com", "test.yaml")
	if err != nil {
		t.Fatalf("GetPageHash (empty): %v", err)
	}
	if h != "" {
		t.Fatalf("expected empty hash, got %q", h)
	}

	err = s.UpdatePageHash("http://example.com", "test.yaml", "abc123")
	if err != nil {
		t.Fatalf("UpdatePageHash: %v", err)
	}

	h, err = s.GetPageHash("http://example.com", "test.yaml")
	if err != nil || h != "abc123" {
		t.Fatalf("GetPageHash: %q, %v", h, err)
	}

	// Discarded
	d, err := s.IsDiscarded("http://bad.com", "test.yaml")
	if err != nil || d {
		t.Fatalf("IsDiscarded (should be false): %v, %v", d, err)
	}

	err = s.MarkDiscarded("http://bad.com", "test.yaml")
	if err != nil {
		t.Fatalf("MarkDiscarded: %v", err)
	}

	d, err = s.IsDiscarded("http://bad.com", "test.yaml")
	if err != nil || !d {
		t.Fatalf("IsDiscarded (should be true): %v, %v", d, err)
	}

	// Minion status
	err = s.SetMinionStatus("test.yaml", true)
	if err != nil {
		t.Fatalf("SetMinionStatus: %v", err)
	}
	if !s.GetMinionStatus("test.yaml") {
		t.Fatal("GetMinionStatus should be true")
	}

	activeMinions, _ := s.GetActiveMinions()
	if !activeMinions["test.yaml"] {
		t.Fatal("GetActiveMinions should include test.yaml")
	}

	// Run queue
	err = s.QueueRun("test.yaml")
	if err != nil {
		t.Fatalf("QueueRun: %v", err)
	}
	queue, _ := s.GetRunQueue()
	if len(queue) != 1 || queue[0] != "test.yaml" {
		t.Fatalf("GetRunQueue: %v", queue)
	}
	err = s.DequeueRun("test.yaml")
	if err != nil {
		t.Fatalf("DequeueRun: %v", err)
	}

	// Abort queue
	err = s.QueueAbort("test.yaml")
	if err != nil {
		t.Fatalf("QueueAbort: %v", err)
	}
	abortQueue, _ := s.GetAbortQueue()
	if len(abortQueue) != 1 {
		t.Fatalf("GetAbortQueue: %v", abortQueue)
	}

	// Active jobs
	err = s.MarkJobActive("test.yaml")
	if err != nil {
		t.Fatalf("MarkJobActive: %v", err)
	}
	jobs, _ := s.GetActiveJobs()
	if !jobs["test.yaml"] {
		t.Fatal("GetActiveJobs should include test.yaml")
	}
	err = s.MarkJobDone("test.yaml")
	if err != nil {
		t.Fatalf("MarkJobDone: %v", err)
	}
	jobs, _ = s.GetActiveJobs()
	if jobs["test.yaml"] {
		t.Fatal("GetActiveJobs should NOT include test.yaml after MarkJobDone")
	}

	// Clear minion state
	deleted, err := s.ClearMinionState("test.yaml")
	if err != nil {
		t.Fatalf("ClearMinionState: %v", err)
	}
	if deleted == 0 {
		t.Fatal("expected some deleted records")
	}
}
