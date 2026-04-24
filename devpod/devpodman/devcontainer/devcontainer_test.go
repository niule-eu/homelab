package devcontainer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateWorkspaceMountRequiresWorkspaceFolder(t *testing.T) {
	t.Run("returns error when workspaceMount set without workspaceFolder", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"workspaceMount": {"source": "src", "target": "/workspace", "type": "volume"}
		}`)

		_, _, _, err := Load(data)
		if err == nil {
			t.Fatal("expected error when workspaceMount is set without workspaceFolder")
		}
	})

	t.Run("succeeds when workspaceMount and workspaceFolder both set", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"workspaceMount": {"source": "src", "target": "/workspace", "type": "volume"},
			"workspaceFolder": "/workspace"
		}`)

		_, _, commonProps, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if commonProps.WorkspaceFolder == nil || *commonProps.WorkspaceFolder != "/workspace" {
			t.Fatalf("expected workspaceFolder '/workspace', got %v", commonProps.WorkspaceFolder)
		}
	})

	t.Run("succeeds when neither workspaceMount nor workspaceFolder set", func(t *testing.T) {
		data := []byte(`{"image": "alpine:latest"}`)

		_, _, _, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateBuildPathsAreRelative(t *testing.T) {
	t.Run("returns error when build.dockerfile is absolute", func(t *testing.T) {
		data := []byte(`{
			"build": {"dockerfile": "/absolute/Dockerfile"}
		}`)

		_, _, _, err := Load(data)
		if err == nil {
			t.Fatal("expected error when build.dockerfile is an absolute path")
		}
	})

	t.Run("returns error when build.context is absolute", func(t *testing.T) {
		data := []byte(`{
			"build": {"dockerfile": "Dockerfile", "context": "/absolute/context"}
		}`)

		_, _, _, err := Load(data)
		if err == nil {
			t.Fatal("expected error when build.context is an absolute path")
		}
	})

	t.Run("succeeds when build.dockerfile and context are relative", func(t *testing.T) {
		data := []byte(`{
			"build": {"dockerfile": "Dockerfile", "context": "."}
		}`)

		_, _, _, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("succeeds when build.context is omitted", func(t *testing.T) {
		data := []byte(`{
			"build": {"dockerfile": "Dockerfile"}
		}`)

		_, _, _, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateWorkspaceFolderIsAbsolute(t *testing.T) {
	t.Run("returns error when workspaceFolder is relative", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"workspaceFolder": "relative/path"
		}`)

		_, _, _, err := Load(data)
		if err == nil {
			t.Fatal("expected error when workspaceFolder is a relative path")
		}
	})

	t.Run("succeeds when workspaceFolder is absolute", func(t *testing.T) {
		data := []byte(`{
			"image": "alpine:latest",
			"workspaceFolder": "/workspace"
		}`)

		_, _, commonProps, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if commonProps.WorkspaceFolder == nil || *commonProps.WorkspaceFolder != "/workspace" {
			t.Fatalf("expected workspaceFolder '/workspace', got %v", commonProps.WorkspaceFolder)
		}
	})

	t.Run("succeeds when workspaceFolder is omitted", func(t *testing.T) {
		data := []byte(`{"image": "alpine:latest"}`)

		_, _, _, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
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
		{"registry with repo without tag", "docker.io/library/nginx", false},
		{"image with digest", "alpine@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", false},
		{"image with tag and digest", "golang:1.22@sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789", false},
		{"multi-level path repo", "mcr.microsoft.com/devcontainers/go", false},
		{"empty image", "", true},
		{"leading whitespace", " golang:1.22", true},
		{"trailing whitespace", "golang:1.22 ", true},
		{"invalid digest format", "alpine@sha256:xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{"image": %q}`, tt.image))

			_, _, _, err := Load(data)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("loads image-based devcontainer", func(t *testing.T) {
		data := []byte(`{
			"name": "Go Dev",
			"image": "mcr.microsoft.com/devcontainers/go:1",
			"remoteUser": "vscode"
		}`)

		buildProps, imgProps, commonProps, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if buildProps != nil {
			t.Fatal("expected buildProps to be nil for image-based devcontainer")
		}
		if imgProps == nil {
			t.Fatal("expected imgProps to be non-nil")
		}
		if imgProps.Image != "mcr.microsoft.com/devcontainers/go:1" {
			t.Fatalf("expected image 'mcr.microsoft.com/devcontainers/go:1', got %q", imgProps.Image)
		}
		if commonProps.Name == nil || *commonProps.Name != "Go Dev" {
			t.Fatalf("expected name 'Go Dev', got %v", commonProps.Name)
		}
		if commonProps.RemoteUser == nil || *commonProps.RemoteUser != "vscode" {
			t.Fatalf("expected remoteUser 'vscode', got %v", commonProps.RemoteUser)
		}
	})

	t.Run("loads build-based devcontainer", func(t *testing.T) {
		data := []byte(`{
			"name": "Custom Build",
			"build": {
				"dockerfile": "Dockerfile",
				"context": ".",
				"target": "dev"
			}
		}`)

		buildProps, imgProps, commonProps, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if imgProps != nil {
			t.Fatal("expected imgProps to be nil for build-based devcontainer")
		}
		if buildProps == nil {
			t.Fatal("expected buildProps to be non-nil")
		}
		if buildProps.Build.Dockerfile != "Dockerfile" {
			t.Fatalf("expected dockerfile 'Dockerfile', got %q", buildProps.Build.Dockerfile)
		}
		if buildProps.Build.Context == nil || *buildProps.Build.Context != "." {
			t.Fatalf("expected context '.', got %v", buildProps.Build.Context)
		}
		if buildProps.Build.Target == nil || *buildProps.Build.Target != "dev" {
			t.Fatalf("expected target 'dev', got %v", buildProps.Build.Target)
		}
		if commonProps.Name == nil || *commonProps.Name != "Custom Build" {
			t.Fatalf("expected name 'Custom Build', got %v", commonProps.Name)
		}
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		_, _, _, err := Load([]byte(`{invalid json`))
		if err == nil {
			t.Fatal("expected error for invalid json")
		}
	})

	t.Run("returns error for empty json", func(t *testing.T) {
		_, _, _, err := Load([]byte(`{}`))
		if err == nil {
			t.Fatal("expected error for empty devcontainer (neither image nor build)")
		}
	})

	t.Run("returns error for empty image", func(t *testing.T) {
		data := []byte(`{"image": ""}`)
		_, _, _, err := Load(data)
		if err == nil {
			t.Fatal("expected error for empty image")
		}
	})

	t.Run("returns error for empty dockerfile", func(t *testing.T) {
		data := []byte(`{"build": {"dockerfile": ""}}`)
		_, _, _, err := Load(data)
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

		_, _, commonProps, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if commonProps.Mounts == nil || len(*commonProps.Mounts) != 2 {
			t.Fatalf("expected 2 mounts, got %v", commonProps.Mounts)
		}

		mounts := *commonProps.Mounts
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

		_, _, commonProps, err := Load(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if commonProps.ContainerEnv == nil || (*commonProps.ContainerEnv)["FOO"] != "bar" {
			t.Fatalf("expected containerEnv FOO=bar, got %v", commonProps.ContainerEnv)
		}
		if commonProps.RemoteEnv == nil || (*commonProps.RemoteEnv)["BAZ"] != "qux" {
			t.Fatalf("expected remoteEnv BAZ=qux, got %v", commonProps.RemoteEnv)
		}
	})

	t.Run("loads from file", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "devcontainer.json")
		content := `{
			"image": "golang:1.22",
			"name": "File Test"
		}`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		fileData, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		buildProps, imgProps, commonProps, err := Load(fileData)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if buildProps != nil {
			t.Fatal("expected buildProps to be nil")
		}
		if imgProps == nil || imgProps.Image != "golang:1.22" {
			t.Fatalf("expected image 'golang:1.22', got %v", imgProps)
		}
		if commonProps.Name == nil || *commonProps.Name != "File Test" {
			t.Fatalf("expected name 'File Test', got %v", commonProps.Name)
		}
	})
}
