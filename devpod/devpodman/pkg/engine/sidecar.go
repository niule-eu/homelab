package engine

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"
)

const (
	// CodeServerPort is the port code-server listens on inside the sidecar.
	CodeServerPort = 8080

	// CodeServerHostPort is the port mapped to the host.
	CodeServerHostPort = 8090

	// PodmanRemoteVersion is the version of podman-remote to bundle.
	PodmanRemoteVersion = "5.8.2"

	// CodeServerVersion is the version of code-server to use.
	CodeServerVersion = "4.98.2"
)

//go:embed assets/Containerfile
var containerfileTemplate string

//go:embed assets/devpodman-shell
var devpodmanShellScript string

//go:embed assets/settings.json
var settingsJSON string

// ContainerfileTemplateData holds the variables for the Containerfile template.
type ContainerfileTemplateData struct {
	PodmanRemoteVersion string
	CodeServerVersion   string
	UserUID             int
	UserGID             int
}

// ImageTag returns the tag for the sidecar image given the host UID.
func ImageTag(uid int) string {
	return fmt.Sprintf("devpodman-code-server:%s-%d", CodeServerVersion, uid)
}

// RenderContainerfile renders the Containerfile template with the given UID/GID.
func RenderContainerfile(uid, gid int) (string, error) {
	tmpl, err := template.New("Containerfile").Parse(containerfileTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse Containerfile template: %w", err)
	}

	data := ContainerfileTemplateData{
		PodmanRemoteVersion: PodmanRemoteVersion,
		CodeServerVersion:   CodeServerVersion,
		UserUID:             uid,
		UserGID:             gid,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render Containerfile: %w", err)
	}

	return buf.String(), nil
}

// RenderConnectionsConfig generates the podman-connections.json content
// for the sidecar to connect back to the host podman socket.
func RenderConnectionsConfig(socketPath string) string {
	cfg := map[string]any{
		"Connection": map[string]any{
			"Default": "host",
			"Connections": map[string]any{
				"host": map[string]any{
					"URI": "unix://" + socketPath,
				},
			},
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "{}"
	}

	return string(data)
}
