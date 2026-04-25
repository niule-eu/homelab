package podman

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	t.Run("fails when socket does not exist", func(t *testing.T) {
		cfg := &Config{
			SocketPath: "/nonexistent/podman.sock",
			Timeout:    DefaultTimeout,
		}

		_, err := NewClient(context.Background(), cfg)
		if err == nil {
			t.Fatal("expected error for nonexistent socket")
		}
	})
}

func TestClientCtx(t *testing.T) {
	t.Run("returns context with connection", func(t *testing.T) {
		cfg := &Config{
			SocketPath: "/run/user/1000/podman/podman.sock",
			Timeout:    DefaultTimeout,
		}

		client, err := NewClient(context.Background(), cfg)
		if err != nil {
			t.Skipf("podman socket not available: %v", err)
		}

		ctx := client.Ctx()
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
	})
}

func TestClientConfig(t *testing.T) {
	t.Run("returns original config", func(t *testing.T) {
		cfg := &Config{
			SocketPath: "/run/user/1000/podman/podman.sock",
			Timeout:    DefaultTimeout,
		}

		client, err := NewClient(context.Background(), cfg)
		if err != nil {
			t.Skipf("podman socket not available: %v", err)
		}

		got := client.Config()
		if got != cfg {
			t.Fatal("expected same config pointer")
		}
	})
}
