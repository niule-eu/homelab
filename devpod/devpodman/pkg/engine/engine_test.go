package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/bindings/pods"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/docker/docker/api/types/container"
	"github.com/niule-eu/devpodman/internal/podman"
	"github.com/niule-eu/devpodman/pkg/devcontainer"
	"github.com/niule-eu/devpodman/pkg/model"
	podmanconfig "go.podman.io/common/pkg/config"
)

func TestNewEngine(t *testing.T) {
	engine := New("/run/podman/podman.sock")
	if engine == nil {
		t.Fatal("New returned nil")
	}
}

func TestPlay_ReturnsCompound(t *testing.T) {
	engine := New("/run/podman/podman.sock")

	cfg := &devcontainer.ResolvedConfig{
		Image: &model.ImageContainer{Image: "alpine:latest"},
		Common: &model.DevContainerCommon{
			Name: "test-project",
		},
	}

	compound, err := engine.Play(context.Background(), cfg, "/tmp/test")
	if err != nil {
		t.Fatalf("Play returned error: %v", err)
	}
	if len(compound.Effects) == 0 {
		t.Fatal("expected non-empty effects list")
	}
}

func TestDown_ReturnsCompound(t *testing.T) {
	engine := New("/run/podman/podman.sock")

	compound, err := engine.Down(context.Background(), "test-pod", false)
	if err != nil {
		t.Fatalf("Down returned error: %v", err)
	}
	if len(compound.Effects) == 0 {
		t.Fatal("expected non-empty effects list")
	}
}

func TestDerivePodName(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *devcontainer.ResolvedConfig
		workspaceDir string
		want         string
	}{
		{
			name: "uses config name",
			cfg: &devcontainer.ResolvedConfig{
				Common: &model.DevContainerCommon{Name: "my-app"},
			},
			workspaceDir: "/some/path",
			want:         "devpodman-my-app",
		},
		{
			name: "falls back to workspace dir basename",
			cfg: &devcontainer.ResolvedConfig{
				Common: &model.DevContainerCommon{},
			},
			workspaceDir: "/home/user/my-project",
			want:         "devpodman-my-project",
		},
		{
			name: "preserves dots and underscores from config name",
			cfg: &devcontainer.ResolvedConfig{
				Common: &model.DevContainerCommon{Name: "my.app_name"},
			},
			workspaceDir: "/tmp",
			want:         "devpodman-my.app_name",
		},
		{
			name: "preserves uppercase from config name",
			cfg: &devcontainer.ResolvedConfig{
				Common: &model.DevContainerCommon{Name: "MyApp"},
			},
			workspaceDir: "/tmp",
			want:         "devpodman-MyApp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DerivePodName(tt.cfg, tt.workspaceDir)
			if got != tt.want {
				t.Errorf("DerivePodName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlay_Integration(t *testing.T) {
	conn := testConn(t)

	socketPath, err := podmanSocketPath(t)
	if err != nil {
		t.Skipf("cannot determine socket path: %v", err)
	}

	podName := "devpodman-integration-play"
	cfg := &devcontainer.ResolvedConfig{
		Image: &model.ImageContainer{Image: "alpine:latest"},
		Common: &model.DevContainerCommon{
			Name:       "integration-play",
			RemoteUser: "root",
			RemoteEnv:  map[string]string{"MY_VAR": "my-value"},
			Customizations: struct {
				Devpodman model.DevpodmanCustomization `json:"devpodman,omitempty"`
			}{
				Devpodman: model.DevpodmanCustomization{
					Workdir: model.DevpodmanWorkdir{
						EmptyVol: true,
					},
				},
			},
		},
		NonCompose: &model.NonComposeBase{
			ContainerEnv: map[string]string{"CT_VAR": "ct-value"},
		},
	}

	eng := New(socketPath)
	compound, err := eng.Play(conn, cfg, "/workspace")
	if err != nil {
		t.Fatalf("Play returned error: %v", err)
	}

	t.Cleanup(func() {
		downCompound, err := eng.Down(conn, podName, false)
		if err != nil {
			t.Logf("cleanup Down failed: %v", err)
			return
		}
		if err := downCompound.Apply(); err != nil {
			t.Logf("cleanup Down Apply failed: %v", err)
		}
	})

	if err := compound.Apply(); err != nil {
		t.Fatalf("Play effects Apply failed: %v", err)
	}

	t.Run("pod exists with correct properties", func(t *testing.T) {
		inspect := inspectPod(t, conn, podName)

		if inspect.Name != podName {
			t.Errorf("pod name = %q, want %q", inspect.Name, podName)
		}
		if inspect.Labels["devpodman/managed"] != "true" {
			t.Errorf("pod label devpodman/managed = %q, want %q", inspect.Labels["devpodman/managed"], "true")
		}
		if inspect.State != "Running" {
			t.Errorf("pod state = %q, want %q", inspect.State, "Running")
		}
		if inspect.NumContainers != 3 {
			t.Errorf("pod numContainers = %d, want 3 (infra + main + sidecar)", inspect.NumContainers)
		}

		containerNames := make(map[string]bool)
		for _, c := range inspect.Containers {
			containerNames[c.Name] = true
		}
		expectedContainers := []string{podName + "-main", podName + "-code-server"}
		for _, name := range expectedContainers {
			if !containerNames[name] {
				t.Errorf("pod missing container %q, has %v", name, containerNames)
			}
		}
	})

	t.Run("main container has correct config", func(t *testing.T) {
		mainName := podName + "-main"
		inspect := inspectContainer(t, conn, mainName)

		if !strings.HasSuffix(inspect.ImageName, "alpine:latest") && inspect.ImageName != "alpine:latest" {
			t.Errorf("main container image = %q, want alpine:latest", inspect.ImageName)
		}
		if !equalStringSlice(inspect.Config.Cmd, []string{"sleep", "infinity"}) {
			t.Errorf("main container cmd = %v, want [sleep infinity]", inspect.Config.Cmd)
		}
		if inspect.Config.WorkingDir != "/workspace" {
			t.Errorf("main container workdir = %q, want /workspace", inspect.Config.WorkingDir)
		}
		if inspect.Config.User != "root" {
			t.Errorf("main container user = %q, want root", inspect.Config.User)
		}

		envMap := envSliceToMap(inspect.Config.Env)
		if envMap["MY_VAR"] != "my-value" {
			t.Errorf("main container env MY_VAR = %q, want my-value", envMap["MY_VAR"])
		}
		if envMap["CT_VAR"] != "ct-value" {
			t.Errorf("main container env CT_VAR = %q, want ct-value", envMap["CT_VAR"])
		}

		hasWorkspaceVol := false
		for _, v := range inspect.Mounts {
			if v.Destination == "/workspace" && v.Name == podName+"-workspace" {
				hasWorkspaceVol = true
			}
		}
		if !hasWorkspaceVol {
			t.Errorf("main container missing workspace volume at /workspace, mounts = %+v", inspect.Mounts)
		}

		if !inspect.State.Running {
			t.Errorf("main container not running, state = %+v", inspect.State)
		}
	})

	t.Run("sidecar container has correct config", func(t *testing.T) {
		sidecarName := podName + "-code-server"
		inspect := inspectContainer(t, conn, sidecarName)

		expectedImageTag := ImageTag(os.Getuid())
		if !strings.HasSuffix(inspect.ImageName, expectedImageTag) && inspect.ImageName != expectedImageTag {
			t.Errorf("sidecar container image = %q, want %s", inspect.ImageName, expectedImageTag)
		}
		expectedCmd := []string{"code-server", "--bind-addr", "0.0.0.0:8080", "/workspace"}
		if !equalStringSlice(inspect.Config.Cmd, expectedCmd) {
			t.Errorf("sidecar container cmd = %v, want %v", inspect.Config.Cmd, expectedCmd)
		}
		if inspect.Config.WorkingDir != "/workspace" {
			t.Errorf("sidecar container workdir = %q, want /workspace", inspect.Config.WorkingDir)
		}

		envMap := envSliceToMap(inspect.Config.Env)
		if envMap["MAIN_CONTAINER_NAME"] != podName+"-main" {
			t.Errorf("sidecar env MAIN_CONTAINER_NAME = %q, want %q", envMap["MAIN_CONTAINER_NAME"], podName+"-main")
		}

		hasWorkspaceVol := false
		hasConnectionsMount := false
		for _, m := range inspect.Mounts {
			if m.Destination == "/workspace" && m.Name == podName+"-workspace" {
				hasWorkspaceVol = true
			}
			if m.Destination == "/home/devpodman/.config/containers" && m.Name == podName+"-connections" {
				hasConnectionsMount = true
			}
		}
		if !hasWorkspaceVol {
			t.Errorf("sidecar container missing workspace volume at /workspace, mounts = %+v", inspect.Mounts)
		}
		if !hasConnectionsMount {
			t.Errorf("sidecar container missing connections volume mount at /home/devpodman/.config/containers")
		}

		if !inspect.State.Running {
			t.Errorf("sidecar container not running, state = %+v", inspect.State)
		}
	})

	t.Run("pod has port mapping 8080->8090", func(t *testing.T) {
		inspect := inspectPod(t, conn, podName)

		hasPortMapping := false
		if inspect.InfraConfig != nil {
			for _, bindings := range inspect.InfraConfig.PortBindings {
				for _, b := range bindings {
					if b.HostPort == "8090" {
						hasPortMapping = true
					}
				}
			}
		}
		if !hasPortMapping {
			t.Errorf("pod missing port mapping 8080->8090, portBindings = %+v", inspect.InfraConfig.PortBindings)
		}
	})

	t.Run("sidecar image exists", func(t *testing.T) {
		expectedTag := ImageTag(os.Getuid())
		exists, err := images.Exists(conn, expectedTag, nil)
		if err != nil {
			t.Fatalf("images.Exists failed: %v", err)
		}
		if !exists {
			t.Errorf("sidecar image %q does not exist", expectedTag)
		}
	})

	t.Run("connections volume has correct content", func(t *testing.T) {
		volName := podName + "-connections"
		content := readVolumeFile(t, conn, volName, "podman-connections.json")

		var parsed podmanconfig.ConnectionsFile
		if err := json.Unmarshal(content, &parsed); err != nil {
			t.Fatalf("podman-connections.json is not valid ConnectionsFile JSON: %v\ncontent: %s", err, content)
		}

		if parsed.Connection.Default != "host" {
			t.Errorf("connection.default = %q, want host", parsed.Connection.Default)
		}

		dest, ok := parsed.Connection.Connections["host"]
		if !ok {
			t.Fatal("expected connection 'host' not found")
		}
		if !strings.HasPrefix(dest.URI, "unix://") {
			t.Errorf("connection URI = %q, want unix:// prefix", dest.URI)
		}
	})

	t.Run("workspace volume shared between containers", func(t *testing.T) {
		mainName := podName + "-main"
		sidecarName := podName + "-code-server"

		output := execContainer(t, conn, mainName, []string{"sh", "-c", "echo hello-from-workspace > /workspace/testfile && cat /workspace/testfile"})
		if !strings.Contains(output, "hello-from-workspace") {
			t.Fatalf("failed to write to workspace in main container, output: %s", output)
		}

		sidecarOutput := execContainer(t, conn, sidecarName, []string{"cat", "/workspace/testfile"})
		if strings.TrimSpace(sidecarOutput) != "hello-from-workspace" {
			t.Errorf("workspace file not shared to sidecar, got %q, want hello-from-workspace", strings.TrimSpace(sidecarOutput))
		}
	})

	t.Run("code-server responds on host port", func(t *testing.T) {
		client := &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		var resp *http.Response
		var err error
		for i := 0; i < 5; i++ {
			resp, err = client.Get("http://10.89.0.1:8090/")
			if err == nil {
				break
			}
			time.Sleep(12 * time.Second)
		}
		if err != nil {
			t.Fatalf("failed to reach code-server after retries: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
			t.Errorf("expected 200 or 302, got %d", resp.StatusCode)
		}
	})
}

func inspectPod(t *testing.T, conn EngineConnection, name string) *define.InspectPodData {
	t.Helper()
	report, err := pods.Inspect(conn, name, nil)
	if err != nil {
		t.Fatalf("failed to inspect pod %q: %v", name, err)
	}
	return report.InspectPodData
}

func inspectContainer(t *testing.T, conn EngineConnection, name string) *define.InspectContainerData {
	t.Helper()
	report, err := containers.Inspect(conn, name, nil)
	if err != nil {
		t.Fatalf("failed to inspect container %q: %v", name, err)
	}
	return report
}

func readVolumeFile(t *testing.T, conn EngineConnection, volName, filePath string) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := volumes.Export(conn, volName, &buf); err != nil {
		t.Fatalf("failed to export volume %q: %v", volName, err)
	}

	tr := tar.NewReader(&buf)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error reading tar from volume %q: %v", volName, err)
		}
		if header.Name == filePath {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("failed to read file %q from volume %q: %v", filePath, volName, err)
			}
			return data
		}
	}
	t.Fatalf("file %q not found in volume %q", filePath, volName)
	return nil
}

func execContainer(t *testing.T, conn EngineConnection, name string, cmd []string) string {
	t.Helper()

	execID, err := containers.ExecCreate(conn, name, &handlers.ExecCreateConfig{
		ExecOptions: container.ExecOptions{
			Cmd:          cmd,
			AttachStdout: true,
			AttachStderr: true,
		},
	})
	if err != nil {
		t.Fatalf("ExecCreate failed: %v", err)
	}

	var stdout bytes.Buffer
	opts := &containers.ExecStartAndAttachOptions{}
	opts = opts.WithAttachOutput(true)
	opts = opts.WithAttachError(true)
	opts = opts.WithOutputStream(&stdout)
	opts = opts.WithErrorStream(&stdout)
	if err := containers.ExecStartAndAttach(conn, execID, opts); err != nil {
		t.Fatalf("ExecStartAndAttach failed: %v", err)
	}
	return stdout.String()
}

func envSliceToMap(env []string) map[string]string {
	result := make(map[string]string)
	for _, e := range env {
		if idx := strings.Index(e, "="); idx >= 0 {
			result[e[:idx]] = e[idx+1:]
		}
	}
	return result
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func podmanSocketPath(t *testing.T) (string, error) {
	t.Helper()
	cfg, err := podman.LoadConfig()
	if err != nil {
		return "", err
	}
	return cfg.SocketPath, nil
}
