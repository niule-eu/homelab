package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"testing"

	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/niule-eu/devpodman/internal/podman"
	"github.com/niule-eu/devpodman/pkg/effects"
)

func testConn(t *testing.T) EngineConnection {
	t.Helper()
	cfg, err := podman.LoadConfig()
	if err != nil {
		t.Skipf("podman not available: %v", err)
	}
	client, err := podman.NewClient(context.Background(), cfg)
	if err != nil {
		t.Skipf("podman connection failed: %v", err)
	}
	return client.Ctx()
}

func TestCreatePodEffect_Apply(t *testing.T) {
	conn := testConn(t)
	t.Cleanup(func() {
		_ = NewRemovePodEffect(conn, "test-effects-pod").Apply()
	})

	eff := NewCreatePodEffect(conn, "test-effects-pod", map[string]string{"io.podman.annotations.userns": "keep-id"}, map[string]string{"devpodman/managed": "true"})
	err := eff.Apply()
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
}

func TestRemovePodEffect_Apply(t *testing.T) {
	conn := testConn(t)

	t.Run("removes existing pod", func(t *testing.T) {
		// Create a pod first
		createEff := NewCreatePodEffect(conn, "test-remove-pod", nil, nil)
		if createErr := createEff.Apply(); createErr != nil {
			t.Fatalf("failed to create test pod: %v", createErr)
		}

		eff := NewRemovePodEffect(conn, "test-remove-pod")
		if rmErr := eff.Apply(); rmErr != nil {
			t.Fatalf("Apply() failed: %v", rmErr)
		}
	})

	t.Run("returns nil for nonexistent pod", func(t *testing.T) {
		eff := NewRemovePodEffect(conn, "test-nonexistent-pod-xyz")
		err := eff.Apply()
		if err != nil {
			t.Fatalf("expected nil for nonexistent pod, got: %v", err)
		}
	})
}

func TestStartPodEffect_Apply(t *testing.T) {
	conn := testConn(t)
	t.Cleanup(func() {
		_ = NewRemovePodEffect(conn, "test-start-pod").Apply()
	})

	createEff := NewCreatePodEffect(conn, "test-start-pod", nil, nil)
	if err := createEff.Apply(); err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}

	eff := NewStartPodEffect(conn, "test-start-pod")
	err := eff.Apply()
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
}

func TestVolumeImportEffect_Apply(t *testing.T) {
	conn := testConn(t)
	t.Cleanup(func() {
		_ = NewRemoveVolumeEffect(conn, "test-import-vol").Apply()
	})

	tarData := makeTarFile("test.txt", []byte("hello world"))

	eff := NewVolumeImportEffect(conn, "test-import-vol", tarData)
	err := eff.Apply()
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
}

func TestRemoveVolumeEffect_Apply(t *testing.T) {
	conn := testConn(t)

	t.Run("removes existing volume", func(t *testing.T) {
		tarData := makeTarFile("test.txt", []byte("hello"))
		importEff := NewVolumeImportEffect(conn, "test-remove-vol", tarData)
		if err := importEff.Apply(); err != nil {
			t.Fatalf("failed to create test volume: %v", err)
		}

		eff := NewRemoveVolumeEffect(conn, "test-remove-vol")
		err := eff.Apply()
		if err != nil {
			t.Fatalf("Apply() failed: %v", err)
		}
	})

	t.Run("returns nil for nonexistent volume", func(t *testing.T) {
		eff := NewRemoveVolumeEffect(conn, "test-nonexistent-vol-xyz")
		err := eff.Apply()
		if err != nil {
			t.Fatalf("expected nil for nonexistent volume, got: %v", err)
		}
	})
}

func TestCreateContainerEffect_Apply(t *testing.T) {
	conn := testConn(t)
	t.Cleanup(func() {
		_ = NewRemovePodEffect(conn, "test-ct-pod").Apply()
	})

	createEff := NewCreatePodEffect(conn, "test-ct-pod", nil, nil)
	if err := createEff.Apply(); err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}

	spec := specgen.NewSpecGenerator("alpine:latest", false)
	spec.Name = "test-ct"
	spec.Pod = "test-ct-pod"
	spec.Command = []string{"sleep", "infinity"}

	eff := NewCreateContainerEffect(conn, spec)
	err := eff.Apply()
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
}

func TestStartContainerEffect_Apply(t *testing.T) {
	conn := testConn(t)
	t.Cleanup(func() {
		_ = NewRemovePodEffect(conn, "test-start-ct-pod").Apply()
	})

	createEff := NewCreatePodEffect(conn, "test-start-ct-pod", nil, nil)
	if err := createEff.Apply(); err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}

	spec := specgen.NewSpecGenerator("alpine:latest", false)
	spec.Name = "test-start-ct"
	spec.Pod = "test-start-ct-pod"
	spec.Command = []string{"sleep", "infinity"}

	ctEff := NewCreateContainerEffect(conn, spec)
	if err := ctEff.Apply(); err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	eff := NewStartContainerEffect(conn, "test-start-ct")
	err := eff.Apply()
	if err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
}

func TestEngineEffectsImplementEffectInterface(t *testing.T) {
	var _ effects.Effect = NewBuildImageEffect(nil, ".", "Dockerfile", "img:latest", nil)
	var _ effects.Effect = NewCreatePodEffect(nil, "pod", nil, nil)
	var _ effects.Effect = NewCreateContainerEffect(nil, specgen.NewSpecGenerator("img", false))
	var _ effects.Effect = NewStartContainerEffect(nil, "ct")
	var _ effects.Effect = NewStartPodEffect(nil, "pod")
	var _ effects.Effect = NewRemovePodEffect(nil, "pod")
	var _ effects.Effect = NewVolumeImportEffect(nil, "vol", nil)
	var _ effects.Effect = NewRemoveVolumeEffect(nil, "vol")
}

func makeTarFile(name string, content []byte) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(content)),
	}
	tw.WriteHeader(hdr)
	tw.Write(content)
	tw.Close()
	return buf.Bytes()
}
