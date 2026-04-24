package podman

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

const (
	DefaultSocketPath = "/run/podman/podman.sock"
	DefaultTimeout    = 10 * time.Second
)

// Config holds podman connection settings.
type Config struct {
	SocketPath string        `koanf:"socketpath"`
	Timeout    time.Duration `koanf:"timeout"`
}

// LoadConfig reads podman configuration from environment variables
// with sensible defaults. Environment variables use the DEVPODMAN_
// prefix (e.g., DEVPODMAN_SOCKETPATH).
func LoadConfig() (*Config, error) {
	k := koanf.New(".")

	defaults := map[string]any{
		"socketpath": defaultSocketPath(),
		"timeout":    DefaultTimeout.String(),
	}
	err := k.Load(confmap.Provider(defaults, "."), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load defaults: %w", err)
	}

	envProvider := env.Provider("DEVPODMAN_", ".", func(s string) string {
		key := strings.TrimPrefix(s, "DEVPODMAN_")
		return strings.ToLower(key)
	})
	err = k.Load(envProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load env vars: %w", err)
	}

	var cfg Config
	err = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"})
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("podman socket not found: set DEVPODMAN_SOCKETPATH or ensure podman socket is enabled and running")
	}

	info, err := os.Stat(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("podman socket not accessible: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("podman socket path is not a unix socket: %s", cfg.SocketPath)
	}

	return &cfg, nil
}

// defaultSocketPath returns the appropriate default socket for the current user.
func defaultSocketPath() string {
	if os.Geteuid() == 0 {
		return defaultRootfulSocket()
	}
	socketPath, _ := defaultRootlessSocket()
	return socketPath
}

// defaultRootfulSocket returns the well-known rootful podman socket path.
func defaultRootfulSocket() string {
	return DefaultSocketPath
}

// defaultRootlessSocket discovers the rootless podman socket via XDG.
func defaultRootlessSocket() (string, error) {
	return xdg.SearchRuntimeFile("podman/podman.sock")
}

// ConnectionURI builds the unix:// URI string from the config.
func (c *Config) ConnectionURI() string {
	return "unix://" + c.SocketPath
}
