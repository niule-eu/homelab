package devcontainer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadImageContainer(t *testing.T) {
	t.Run("loads image-based devcontainer", func(t *testing.T) {
		data := []byte(`{
			"name": "Go-Dev",
			"image": "mcr.microsoft.com/devcontainers/go:1",
			"remoteUser": "vscode"
		}`)

		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build != nil {
			t.Fatal("expected Build to be nil for image-based devcontainer")
		}
		if cfg.Image == nil {
			t.Fatal("expected Image to be non-nil")
		}
		if cfg.Image.Image != "mcr.microsoft.com/devcontainers/go:1" {
			t.Fatalf("expected image 'mcr.microsoft.com/devcontainers/go:1', got %q", cfg.Image.Image)
		}
		if cfg.Common == nil || cfg.Common.Name != "Go-Dev" {
			t.Fatalf("expected name 'Go-Dev', got %v", cfg.Common)
		}
		if cfg.Common.RemoteUser != "vscode" {
			t.Fatalf("expected remoteUser 'vscode', got %q", cfg.Common.RemoteUser)
		}
	})
}

func TestLoadBuildContainer(t *testing.T) {
	t.Run("loads build-based devcontainer", func(t *testing.T) {
		data := []byte(`{
			"name": "Custom-Build",
			"build": {
				"dockerfile": "Dockerfile",
				"context": ".",
				"target": "dev"
			}
		}`)

		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Image != nil {
			t.Fatal("expected Image to be nil for build-based devcontainer")
		}
		if cfg.Build == nil {
			t.Fatal("expected Build to be non-nil")
		}
		if cfg.Build.Build.Dockerfile != "Dockerfile" {
			t.Fatalf("expected dockerfile 'Dockerfile', got %q", cfg.Build.Build.Dockerfile)
		}
		if cfg.Build.Build.Context != "." {
			t.Fatalf("expected context '.', got %q", cfg.Build.Build.Context)
		}
		if cfg.Build.Build.Target != "dev" {
			t.Fatalf("expected target 'dev', got %q", cfg.Build.Build.Target)
		}
		if cfg.Common == nil || cfg.Common.Name != "Custom-Build" {
			t.Fatalf("expected name 'Custom-Build', got %v", cfg.Common)
		}
	})
}

func TestLoadPriority(t *testing.T) {
	t.Run("both image and build provided — dockerfile wins", func(t *testing.T) {
		data := []byte(`{
			"image": "golang:1.22",
			"build": {
				"dockerfile": "Dockerfile"
			}
		}`)

		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build == nil {
			t.Fatal("expected Build to win over Image (dockerfile priority)")
		}
		if cfg.Image != nil {
			t.Fatal("expected Image to be nil when dockerfile wins")
		}
	})

	t.Run("neither image nor build — error", func(t *testing.T) {
		data := []byte(`{"name": "no container"}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when neither image nor build provided")
		}
	})
}

func TestValidateImageReference(t *testing.T) {
	tests := []struct {
		name    string
		image   string
		wantErr bool
	}{
		{"simple repo with tag", "golang:1.22", false},
		{"simple repo without tag", "alpine", false},
		{"registry with repo and tag", "mcr.microsoft.com/devcontainers/go:1.21", false},
		{"image with digest", "alpine@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", false},
		{"empty image", "", true},
		{"leading whitespace", " golang:1.22", true},
		{"trailing whitespace", "golang:1.22 ", true},
		{"invalid digest format", "alpine@sha256:xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"image": %q}`, tt.image))
			_, err := Load(data)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBuildPathsAreRelative(t *testing.T) {
	t.Run("returns error when build.dockerfile is absolute", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": "/absolute/Dockerfile"}}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when build.dockerfile is an absolute path")
		}
	})

	t.Run("returns error when build.context is absolute", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": "Dockerfile", "context": "/absolute/context"}}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error when build.context is an absolute path")
		}
	})

	t.Run("succeeds when build.dockerfile and context are relative", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": "Dockerfile", "context": "."}}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build == nil {
			t.Fatal("expected Build to be non-nil")
		}
	})
}

func TestLoadEdgeCases(t *testing.T) {
	t.Run("returns error for invalid json", func(t *testing.T) {
		_, err := Load([]byte(`{invalid json`))
		if err == nil {
			t.Fatal("expected error for invalid json")
		}
	})

	t.Run("returns error for empty json", func(t *testing.T) {
		_, err := Load([]byte(`{}`))
		if err == nil {
			t.Fatal("expected error for empty devcontainer (neither image nor build)")
		}
	})

	t.Run("returns error for empty image", func(t *testing.T) {
		data := []byte(`{"image": ""}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error for empty image")
		}
	})

	t.Run("returns error for empty dockerfile", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": ""}}`)
		_, err := Load(data)
		if err == nil {
			t.Fatal("expected error for empty dockerfile")
		}
	})

	t.Run("parses mounts", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"mounts": [
				{"source": "data", "target": "/data", "type": "volume"},
				{"source": "/host/path", "target": "/container/path", "type": "bind"}
			]
		}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.NonCompose == nil || len(cfg.NonCompose.Mounts) != 2 {
			t.Fatalf("expected 2 mounts, got %v", cfg.NonCompose)
		}
		mounts := cfg.NonCompose.Mounts
		if mounts[0].Source != "data" || mounts[0].Target != "/data" {
			t.Fatalf("unexpected first mount: %+v", mounts[0])
		}
		if mounts[1].Source != "/host/path" || mounts[1].Target != "/container/path" {
			t.Fatalf("unexpected second mount: %+v", mounts[1])
		}
	})

	t.Run("parses containerEnv and remoteEnv", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"containerEnv": {"FOO": "bar"},
			"remoteEnv": {"BAZ": "qux"}
		}`)
		cfg, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.NonCompose == nil || cfg.NonCompose.ContainerEnv["FOO"] != "bar" {
			t.Fatalf("expected containerEnv FOO=bar, got %v", cfg.NonCompose)
		}
		if cfg.Common == nil || cfg.Common.RemoteEnv["BAZ"] != "qux" {
			t.Fatalf("expected remoteEnv BAZ=qux, got %v", cfg.Common)
		}
	})

	t.Run("loads from file", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "devcontainer.json")
		content := `{
			"image": "golang:1.22",
			"name": "File-Test"
		}`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		fileData, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(fileData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Build != nil {
			t.Fatal("expected Build to be nil")
		}
		if cfg.Image == nil || cfg.Image.Image != "golang:1.22" {
			t.Fatalf("expected image 'golang:1.22', got %v", cfg.Image)
		}
		if cfg.Common == nil || cfg.Common.Name != "File-Test" {
			t.Fatalf("expected name 'File-Test', got %v", cfg.Common)
		}
	})
}
