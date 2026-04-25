package podman

import (
	"fmt"

	types "github.com/containers/podman/v5/pkg/domain/entities/types"

	"github.com/containers/podman/v5/pkg/bindings/pods"
	"github.com/containers/podman/v5/pkg/specgen"
)

// CreatePod creates a new pod with the given name and labels.
func (c *Client) CreatePod(name string, labels map[string]string) (*types.PodCreateReport, error) {
	s := specgen.NewPodSpecGenerator()
	s.Name = name
	s.Labels = labels

	report, err := pods.CreatePodFromSpec(c.ctx, &types.PodSpec{PodSpecGen: *s})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod %q: %w", name, err)
	}
	return report, nil
}

// RemovePod stops and removes the pod and all its containers.
// Returns nil if the pod does not exist.
func (c *Client) RemovePod(name string) error {
	exists, err := pods.Exists(c.ctx, name, &pods.ExistsOptions{})
	if err != nil {
		return fmt.Errorf("failed to check if pod %q exists: %w", name, err)
	}
	if !exists {
		return nil
	}
	_, err = pods.Remove(c.ctx, name, &pods.RemoveOptions{Force: ptrBool(true)})
	if err != nil {
		return fmt.Errorf("failed to remove pod %q: %w", name, err)
	}
	return nil
}

// PodExists returns true if a pod with the given name exists.
func (c *Client) PodExists(name string) (bool, error) {
	return pods.Exists(c.ctx, name, &pods.ExistsOptions{})
}
