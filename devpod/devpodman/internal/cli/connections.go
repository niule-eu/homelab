package cli

import (
	"context"

	"github.com/niule-eu/devpodman/internal/podman"
)

// NewEngineConnection creates a validated podman connection context
// for use with pkg/engine. The connection is validated by podman.NewClient.
func NewEngineConnection(ctx context.Context, cfg *podman.Config) (context.Context, error) {
	client, err := podman.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return client.Ctx(), nil
}
