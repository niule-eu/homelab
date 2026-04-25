package podman

import (
	"context"
	"fmt"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
)

// Client wraps the podman bindings connection.
type Client struct {
	ctx    context.Context
	config *Config
}

// NewClient creates a new podman client with the given config.
// It validates the connection by attempting a lightweight API call.
func NewClient(ctx context.Context, cfg *Config) (*Client, error) {
	connCtx, err := bindings.NewConnection(ctx, cfg.ConnectionURI())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to podman socket %s: %w", cfg.SocketPath, err)
	}

	// Validate connection with a lightweight API call
	_, err = containers.List(connCtx, &containers.ListOptions{
		All:  ptrBool(true),
		Last: ptrInt(1),
	})
	if err != nil {
		return nil, fmt.Errorf("podman connection validation failed: %w", err)
	}

	return &Client{
		ctx:    connCtx,
		config: cfg,
	}, nil
}

// Ctx returns the context with the embedded podman connection.
// This context must be passed to all podman API calls.
func (c *Client) Ctx() context.Context {
	return c.ctx
}

// Config returns the client configuration.
func (c *Client) Config() *Config {
	return c.config
}

// ListContainers returns all containers.
func (c *Client) ListContainers() ([]types.ListContainer, error) {
	return containers.List(c.ctx, &containers.ListOptions{
		All: ptrBool(true),
	})
}
