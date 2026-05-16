package engine

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/specgenutil"
	"github.com/distribution/reference"
	"github.com/niule-eu/devpodman/pkg/devcontainer"
	"github.com/niule-eu/devpodman/pkg/effects"
	ocispec "github.com/opencontainers/runtime-spec/specs-go"
	nettypes "go.podman.io/common/libnetwork/types"
)

// Engine orchestrates devcontainer pod lifecycle.
type Engine struct {
	socketPath string
}

// New creates a new Engine. The socketPath is the path to the host's
// podman socket, which will be bind-mounted into the sidecar container.
func New(socketPath string) *Engine {
	return &Engine{socketPath: socketPath}
}

// Play returns a Compound effect that creates a devcontainer pod
// with a main container and code-server sidecar.
func (e *Engine) Play(conn EngineConnection, cfg *devcontainer.ResolvedConfig, workspaceDir string) (effects.Compound, error) {
	podName := DerivePodName(cfg, workspaceDir)

	uid := os.Getuid()
	gid := os.Getgid()
	sidecarTag := ImageTag(uid)

	renderedContainerfile, err := RenderContainerfile(uid, gid)
	if err != nil {
		return effects.Compound{}, fmt.Errorf("failed to render Containerfile: %w", err)
	}

	connectionsJSON := RenderConnectionsConfig(e.socketPath)

	var effs []effects.Effect

	effs = append(effs, NewCreateNetworkEffect(conn, devpodmanNetwork))

	tmpDir := filepath.Join(os.TempDir(), "devpodman-"+podName)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return effects.Compound{}, fmt.Errorf("failed to create build context dir: %w", err)
	}
	containerfilePath := filepath.Join(tmpDir, "Containerfile")
	shellPath := filepath.Join(tmpDir, "devpodman-shell")

	effs = append(effs, effects.FileWrite{
		Path:        containerfilePath,
		Content:     []byte(renderedContainerfile),
		Permissions: 0644,
	})
	effs = append(effs, effects.FileWrite{
		Path:        shellPath,
		Content:     []byte(devpodmanShellScript),
		Permissions: 0755,
	})
	settingsPath := filepath.Join(tmpDir, "settings.json")
	effs = append(effs, effects.FileWrite{
		Path:        settingsPath,
		Content:     []byte(settingsJSON),
		Permissions: 0644,
	})

	tagRef, err := reference.ParseNormalizedNamed(sidecarTag)
	if err != nil {
		return effects.Compound{}, fmt.Errorf("failed to parse sidecar image tag: %w", err)
	}
	tagged, ok := tagRef.(reference.NamedTagged)
	if !ok {
		return effects.Compound{}, fmt.Errorf("sidecar image tag is not named/tagged: %s", sidecarTag)
	}
	effs = append(effs, NewBuildImageEffect(conn, tmpDir, "Containerfile", tagged, nil))

	connectionsVolumeName := podName + "-connections"
	effs = append(effs, NewVolumeImportEffect(conn, connectionsVolumeName, createTarForFile("podman-connections.json", []byte(connectionsJSON))))

	effs = append(effs, NewCreatePodEffect(conn, podName, map[string]string{
		"io.podman.annotations.userns": "keep-id",
	}, map[string]string{
		"devpodman/managed": "true",
	}, []nettypes.PortMapping{
		{
			ContainerPort: 8080,
			HostPort:      8090,
			Protocol:      "tcp",
		},
	}))

	mainSpec, err := buildMainContainerSpec(cfg, workspaceDir)
	if err != nil {
		return effects.Compound{}, err
	}
	effs = append(effs, NewCreateContainerEffect(conn, mainSpec))

	sidecarSpec := buildSidecarContainerSpec(cfg, workspaceDir, e.socketPath)
	effs = append(effs, NewCreateContainerEffect(conn, sidecarSpec))

	effs = append(effs, NewStartPodEffect(conn, podName))

	return effects.Compound{Effects: effs, FailFast: true}, nil
}

// Down returns a Compound effect that stops and removes a devcontainer pod.
func (e *Engine) Down(conn EngineConnection, podName string, deleteImages bool) (effects.Compound, error) {
	var effs []effects.Effect

	effs = append(effs, NewRemovePodEffect(conn, podName))

	connectionsVolumeName := podName + "-connections"
	effs = append(effs, NewRemoveVolumeEffect(conn, connectionsVolumeName))

	workspaceVolName := podName + "-workspace"
	effs = append(effs, NewRemoveVolumeEffect(conn, workspaceVolName))

	effs = append(effs, NewRemoveNetworkEffect(conn, devpodmanNetwork))

	_ = deleteImages

	return effects.Compound{Effects: effs, FailFast: true}, nil
}

// DerivePodName returns the deterministic pod name for a config + workspace dir.
// Exported so the CLI can compute the pod name independently.
func DerivePodName(cfg *devcontainer.ResolvedConfig, workspaceDir string) string {
	if cfg.Common != nil && cfg.Common.Name != "" {
		return "devpodman-" + cfg.Common.Name
	}
	return "devpodman-" + filepath.Base(workspaceDir)
}

func resolveMainImage(cfg *devcontainer.ResolvedConfig, podName string) (string, error) {
	if cfg.Image != nil {
		return cfg.Image.Image, nil
	}
	if cfg.Build != nil {
		return podName + "-main", nil
	}
	return "", fmt.Errorf("devcontainer.json must specify 'image' or 'build'")
}

func buildMainContainerSpec(cfg *devcontainer.ResolvedConfig, workspaceDir string) (*specgen.SpecGenerator, error) {
	podName := DerivePodName(cfg, workspaceDir)
	name := podName + "-main"

	mainImage, err := resolveMainImage(cfg, podName)
	if err != nil {
		return nil, err
	}

	useEmptyVol := cfg.Common != nil && cfg.Common.Customizations.Devpodman.Workdir.EmptyVol
	workspaceVolName := podName + "-workspace"

	opts := &entities.ContainerCreateOptions{}
	common.DefineCreateDefaults(opts)

	var runArgs []string
	if cfg.NonCompose != nil {
		runArgs = cfg.NonCompose.RunArgs
	}

	if cfg.Common != nil && cfg.Common.RemoteUser != "" {
		opts.User = cfg.Common.RemoteUser
	} else if cfg.NonCompose != nil && cfg.NonCompose.ContainerUser != "" {
		opts.User = cfg.NonCompose.ContainerUser
	}

	if cfg.NonCompose != nil && cfg.NonCompose.WorkspaceFolder != "" {
		opts.Workdir = cfg.NonCompose.WorkspaceFolder
	}

	envVars := make([]string, 0)
	if cfg.NonCompose != nil {
		for k, v := range cfg.NonCompose.ContainerEnv {
			envVars = append(envVars, k+"="+v)
		}
	}
	if cfg.Common != nil {
		for k, v := range cfg.Common.RemoteEnv {
			envVars = append(envVars, k+"="+v)
		}
	}
	opts.Env = envVars

	if err := ApplyRunArgs(opts, runArgs); err != nil {
		return nil, fmt.Errorf("failed to apply runArgs: %w", err)
	}

	// Convert to SpecGenerator
	spec := specgen.NewSpecGenerator(mainImage, false)
	if err := specgenutil.FillOutSpecGen(spec, opts, nil); err != nil {
		return nil, fmt.Errorf("failed to build container spec: %w", err)
	}

	// Apply pod-specific settings (not controlled by runArgs)
	spec.Name = name
	spec.Pod = podName
	if len(spec.Command) == 0 {
		spec.Command = []string{"sleep", "infinity"}
	}

	// Workspace mount/volume
	if useEmptyVol {
		spec.Volumes = append(spec.Volumes, &specgen.NamedVolume{
			Name: workspaceVolName, Dest: "/workspace",
		})
	} else {
		spec.Mounts = append(spec.Mounts, ocispec.Mount{
			Source:      workspaceDir,
			Destination: "/workspace",
			Type:        "bind",
		})
	}

	// Additional mounts from devcontainer.json
	if cfg.NonCompose != nil {
		for _, m := range cfg.NonCompose.Mounts {
			spec.Mounts = append(spec.Mounts, ocispec.Mount{
				Source:      m.Source,
				Destination: m.Target,
				Type:        m.Type,
			})
		}
	}

	return spec, nil
}

func buildSidecarContainerSpec(cfg *devcontainer.ResolvedConfig, workspaceDir, socketPath string) *specgen.SpecGenerator {
	podName := DerivePodName(cfg, workspaceDir)
	name := podName + "-code-server"
	mainContainerName := podName + "-main"
	connectionsVolumeName := podName + "-connections"
	useEmptyVol := cfg.Common != nil && cfg.Common.Customizations.Devpodman.Workdir.EmptyVol
	workspaceVolName := podName + "-workspace"

	uid := os.Getuid()
	sidecarImage := ImageTag(uid)

	opts := &entities.ContainerCreateOptions{}
	common.DefineCreateDefaults(opts)
	opts.Workdir = "/workspace"
	opts.Env = []string{"MAIN_CONTAINER_NAME=" + mainContainerName}
	if err := ApplyRunArgs(opts, nil); err != nil {
		panic(fmt.Sprintf("unexpected ApplyRunArgs error: %v", err))
	}

	spec := specgen.NewSpecGenerator(sidecarImage, false)
	if err := specgenutil.FillOutSpecGen(spec, opts, nil); err != nil {
		panic(fmt.Sprintf("unexpected FillOutSpecGen error: %v", err))
	}

	spec.Name = name
	spec.Pod = podName
	spec.Command = []string{"code-server", "--bind-addr", "0.0.0.0:8080", "/workspace"}

	spec.Mounts = append(spec.Mounts, ocispec.Mount{
		Source:      socketPath,
		Destination: socketPath,
		Type:        "bind",
	})

	if useEmptyVol {
		spec.Volumes = []*specgen.NamedVolume{
			{Name: workspaceVolName, Dest: "/workspace"},
			{Name: connectionsVolumeName, Dest: "/home/devpodman/.config/containers"},
		}
	} else {
		spec.Mounts = append(spec.Mounts, ocispec.Mount{
			Source:      workspaceDir,
			Destination: "/workspace",
			Type:        "bind",
		})
		spec.Volumes = []*specgen.NamedVolume{
			{Name: connectionsVolumeName, Dest: "/home/devpodman/.config/containers"},
		}
	}

	return spec
}

func createTarForFile(name string, data []byte) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: name,
		Mode: 0644,
		Size: int64(len(data)),
	}
	tw.WriteHeader(hdr)
	tw.Write(data)
	tw.Close()
	return buf.Bytes()
}
