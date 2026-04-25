package podman

import (
	"context"
	"testing"
)

func TestCreatePod(t *testing.T) {
	// t.Skip("requires podman socket")

	cfg, err := LoadConfig()
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Skipf("podman connection failed: %v", err)
	}

	t.Cleanup(func() { _ = client.RemovePod("test-create-pod") })

	ann := map[string]string{"io.podman.annotations.userns": "keep-id"}
	lbl := map[string]string{"devpodman/managed": "true"}

	report, err := client.CreatePod("test-create-pod", ann, lbl)
	if err != nil {
		t.Fatalf("CreatePod failed: %v", err)
	}
	if report.Id == "" {
		t.Fatal("expected non-empty pod ID")
	}

	exists, err := client.PodExists("test-create-pod")
	if err != nil {
		t.Fatalf("PodExists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected pod to exist")
	}
}

func TestRemovePod(t *testing.T) {
	// t.Skip("requires podman socket")

	cfg, err := LoadConfig()
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}
	client, err := NewClient(context.Background(), cfg)
	if err != nil {
		t.Skipf("podman connection failed: %v", err)
	}

	err = client.RemovePod("test-nonexistent-pod")
	if err != nil {
		t.Fatalf("expected nil for non-existent pod, got: %v", err)
	}
}
