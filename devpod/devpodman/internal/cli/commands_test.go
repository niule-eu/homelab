package cli

import (
	"testing"
)

func TestNewDebugCommand(t *testing.T) {
	cmd := NewDebugCommand()
	if cmd == nil {
		t.Fatal("NewDebugCommand returned nil")
	}
	if cmd.Name != "debug" {
		t.Fatalf("expected command name 'debug', got %q", cmd.Name)
	}
}
