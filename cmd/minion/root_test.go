package minion

import "testing"

func TestRootVersion(t *testing.T) {
	if rootCmd.Version != "4.6.0" {
		t.Fatalf("root version = %q, want 4.6.0", rootCmd.Version)
	}
}
