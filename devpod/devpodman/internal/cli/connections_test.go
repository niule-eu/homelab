package cli

import (
	"context"
	"testing"

	"github.com/niule-eu/devpodman/internal/podman"
)

func TestNewEngineConnection(t *testing.T) {
	cfg, err := podman.LoadConfig()
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}

	conn, err := NewEngineConnection(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewEngineConnection failed: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
}
