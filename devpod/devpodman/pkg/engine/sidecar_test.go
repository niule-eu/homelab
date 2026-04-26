package engine

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

func TestImageTag(t *testing.T) {
	tag := ImageTag(1000)
	expected := "devpodman-code-server:4.98.2-1000"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestContainerfileTemplate(t *testing.T) {
	rendered, err := RenderContainerfile(1000, 1000)
	if err != nil {
		t.Fatalf("RenderContainerfile failed: %v", err)
	}

	// Verify versions are substituted
	if !strings.Contains(rendered, "ghcr.io/mgoltzsche/podman:5.8.2-remote") {
		t.Error("expected podman remote version substituted")
	}
	if !strings.Contains(rendered, "docker.io/codercom/code-server:4.98.2") {
		t.Error("expected code-server version substituted")
	}

	// Verify no ARG directives remain
	if strings.Contains(rendered, "ARG ") {
		t.Error("expected no ARG directives in rendered template")
	}

	// Verify UID/GID values are inlined
	if !strings.Contains(rendered, "groupadd -g 1000 devpodman") {
		t.Error("expected USER_GID inlined in groupadd")
	}
	if !strings.Contains(rendered, "useradd -m -u 1000 -g 1000") {
		t.Error("expected USER_UID/USER_GID inlined in useradd")
	}
}

func TestContainerfileTemplate_Parse(t *testing.T) {
	// Verify the raw embedded template parses correctly
	tmpl, err := template.New("Containerfile").Parse(containerfileTemplate)
	if err != nil {
		t.Fatalf("failed to parse Containerfile template: %v", err)
	}

	data := ContainerfileTemplateData{
		PodmanRemoteVersion: "5.8.2",
		CodeServerVersion:   "4.98.2",
		UserUID:             1000,
		UserGID:             1000,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("failed to render template: %v", err)
	}
}

func TestRenderConnectionsConfig(t *testing.T) {
	cfg := RenderConnectionsConfig("/run/user/1000/podman/podman.sock")
	if cfg == "" {
		t.Fatal("expected non-empty config")
	}

	// Verify it's valid JSON
	var v map[string]any
	if err := json.Unmarshal([]byte(cfg), &v); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}

	// Verify socket path is present
	if !strings.Contains(cfg, "unix:///run/user/1000/podman/podman.sock") {
		t.Error("expected socket URI in config")
	}
}

func TestDevpodmanShellEmbedded(t *testing.T) {
	if devpodmanShellScript == "" {
		t.Fatal("expected non-empty devpodman-shell script")
	}
	if !strings.HasPrefix(devpodmanShellScript, "#!/bin/bash") {
		t.Error("expected shebang line in devpodman-shell")
	}
	if !strings.Contains(devpodmanShellScript, "MAIN_CONTAINER_NAME") {
		t.Error("expected MAIN_CONTAINER_NAME reference in devpodman-shell")
	}
}
