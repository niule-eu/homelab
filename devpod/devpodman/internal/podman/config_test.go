package podman

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/adrg/xdg"
)

func TestLoadConfig(t *testing.T) {
	t.Run("returns error when socket not found and no env override", func(t *testing.T) {
		os.Unsetenv("DEVPODMAN_SOCKETPATH")
		os.Unsetenv("DEVPODMAN_TIMEOUT")
		os.Unsetenv("XDG_RUNTIME_DIR")
		xdg.Reload()

		cfg, err := LoadConfig()
		if err == nil {
			if cfg.SocketPath == "" {
				t.Fatal("expected error or valid socket path when podman socket not found")
			}
		}
	})

	t.Run("uses XDG_RUNTIME_DIR for rootless", func(t *testing.T) {
		os.Unsetenv("DEVPODMAN_SOCKETPATH")
		os.Setenv("XDG_RUNTIME_DIR", "/run/user/42")
		xdg.Reload()
		defer func() {
			os.Unsetenv("XDG_RUNTIME_DIR")
			xdg.Reload()
		}()

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error when socket file does not exist")
		}
	})

	t.Run("reads socket path from env", func(t *testing.T) {
		sockPath := t.TempDir() + "/podman.sock"
		listener, err := net.Listen("unix", sockPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}
		defer listener.Close()

		os.Setenv("DEVPODMAN_SOCKETPATH", sockPath)
		defer os.Unsetenv("DEVPODMAN_SOCKETPATH")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.SocketPath != sockPath {
			t.Errorf("expected %s, got %s", sockPath, cfg.SocketPath)
		}
	})

	t.Run("reads timeout from env", func(t *testing.T) {
		sockPath := t.TempDir() + "/podman.sock"
		listener, err := net.Listen("unix", sockPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}
		defer listener.Close()

		os.Setenv("DEVPODMAN_TIMEOUT", "30s")
		defer os.Unsetenv("DEVPODMAN_TIMEOUT")
		os.Setenv("DEVPODMAN_SOCKETPATH", sockPath)
		defer os.Unsetenv("DEVPODMAN_SOCKETPATH")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Timeout != 30*time.Second {
			t.Errorf("expected 30s timeout, got %v", cfg.Timeout)
		}
	})

	t.Run("env var overrides xdg default socket path", func(t *testing.T) {
		sockPath := t.TempDir() + "/podman.sock"
		listener, err := net.Listen("unix", sockPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}
		defer listener.Close()

		os.Setenv("DEVPODMAN_SOCKETPATH", sockPath)
		defer os.Unsetenv("DEVPODMAN_SOCKETPATH")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.SocketPath != sockPath {
			t.Errorf("expected %s, got %s", sockPath, cfg.SocketPath)
		}
	})

	t.Run("rejects non-socket path", func(t *testing.T) {
		regPath := t.TempDir() + "/not-a-socket"
		f, err := os.Create(regPath)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		f.Close()

		os.Setenv("DEVPODMAN_SOCKETPATH", regPath)
		defer os.Unsetenv("DEVPODMAN_SOCKETPATH")

		_, err = LoadConfig()
		if err == nil {
			t.Fatal("expected error when path is not a socket")
		}
	})
}

func TestConfigConnectionURI(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name:     "rootful path",
			cfg:      &Config{SocketPath: DefaultSocketPath},
			expected: "unix:///run/podman/podman.sock",
		},
		{
			name:     "custom path",
			cfg:      &Config{SocketPath: "/tmp/podman-test.sock"},
			expected: "unix:///tmp/podman-test.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := tt.cfg.ConnectionURI()
			if uri != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, uri)
			}
		})
	}
}
