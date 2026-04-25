package engine

import (
	"bytes"
	"fmt"

	"github.com/containers/buildah/define"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/bindings/pods"
	"github.com/containers/podman/v5/pkg/bindings/volumes"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/niule-eu/devpodman/pkg/effects"
)

// BuildImageEffect builds a container image from a Dockerfile.
type BuildImageEffect struct {
	conn       EngineConnection
	ContextDir string
	Dockerfile string
	Tag        string
	BuildArgs  map[string]string
}

func NewBuildImageEffect(conn EngineConnection, contextDir, dockerfile, tag string, buildArgs map[string]string) effects.Effect {
	return BuildImageEffect{conn: conn, ContextDir: contextDir, Dockerfile: dockerfile, Tag: tag, BuildArgs: buildArgs}
}

func (e BuildImageEffect) Apply() error {
	opts := types.BuildOptions{
		BuildOptions: define.BuildOptions{
			Output: e.Tag,
			Args:   e.BuildArgs,
		},
		ContainerFiles: []string{e.Dockerfile},
	}

	_, err := images.Build(e.conn, []string{e.ContextDir}, opts)
	if err != nil {
		return fmt.Errorf("failed to build image %q: %w", e.Tag, err)
	}
	return nil
}

// CreatePodEffect creates a new pod with the given name, annotations, and labels.
type CreatePodEffect struct {
	conn        EngineConnection
	Name        string
	Annotations map[string]string
	Labels      map[string]string
}

func NewCreatePodEffect(conn EngineConnection, name string, annotations, labels map[string]string) effects.Effect {
	return CreatePodEffect{conn: conn, Name: name, Annotations: annotations, Labels: labels}
}

func (e CreatePodEffect) Apply() error {
	s := specgen.NewPodSpecGenerator()
	s.Name = e.Name
	s.Labels = e.Labels

	if v, ok := e.Annotations["io.podman.annotations.userns"]; ok && v == "keep-id" {
		s.Userns.NSMode = specgen.KeepID
	}

	_, err := pods.CreatePodFromSpec(e.conn, &types.PodSpec{PodSpecGen: *s})
	if err != nil {
		return fmt.Errorf("failed to create pod %q: %w", e.Name, err)
	}
	return nil
}

// CreateContainerEffect creates a container using the given spec.
type CreateContainerEffect struct {
	conn EngineConnection
	Spec *specgen.SpecGenerator
}

func NewCreateContainerEffect(conn EngineConnection, spec *specgen.SpecGenerator) effects.Effect {
	return CreateContainerEffect{conn: conn, Spec: spec}
}

func (e CreateContainerEffect) Apply() error {
	_, err := containers.CreateWithSpec(e.conn, e.Spec, nil)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	return nil
}

// StartContainerEffect starts a container by name or ID.
type StartContainerEffect struct {
	conn EngineConnection
	Name string
}

func NewStartContainerEffect(conn EngineConnection, name string) effects.Effect {
	return StartContainerEffect{conn: conn, Name: name}
}

func (e StartContainerEffect) Apply() error {
	err := containers.Start(e.conn, e.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to start container %q: %w", e.Name, err)
	}
	return nil
}

// StartPodEffect starts a pod by name.
type StartPodEffect struct {
	conn EngineConnection
	Name string
}

func NewStartPodEffect(conn EngineConnection, name string) effects.Effect {
	return StartPodEffect{conn: conn, Name: name}
}

func (e StartPodEffect) Apply() error {
	_, err := pods.Start(e.conn, e.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to start pod %q: %w", e.Name, err)
	}
	return nil
}

// RemovePodEffect stops and removes a pod and all its containers.
type RemovePodEffect struct {
	conn EngineConnection
	Name string
}

func NewRemovePodEffect(conn EngineConnection, name string) effects.Effect {
	return RemovePodEffect{conn: conn, Name: name}
}

func (e RemovePodEffect) Apply() error {
	exists, err := pods.Exists(e.conn, e.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to check if pod %q exists: %w", e.Name, err)
	}
	if !exists {
		return nil
	}
	_, err = pods.Remove(e.conn, e.Name, &pods.RemoveOptions{Force: ptrBool(true)})
	if err != nil {
		return fmt.Errorf("failed to remove pod %q: %w", e.Name, err)
	}
	return nil
}

// VolumeImportEffect creates a podman volume and imports a tar stream into it.
type VolumeImportEffect struct {
	conn    EngineConnection
	Name    string
	TarData []byte
}

func NewVolumeImportEffect(conn EngineConnection, name string, tarData []byte) effects.Effect {
	return VolumeImportEffect{conn: conn, Name: name, TarData: tarData}
}

func (e VolumeImportEffect) Apply() error {
	_, err := volumes.Create(e.conn, types.VolumeCreateOptions{Name: e.Name}, nil)
	if err != nil {
		return fmt.Errorf("failed to create volume %q: %w", e.Name, err)
	}

	reader := bytes.NewReader(e.TarData)
	err = volumes.Import(e.conn, e.Name, reader)
	if err != nil {
		return fmt.Errorf("failed to import data into volume %q: %w", e.Name, err)
	}

	return nil
}

// RemoveVolumeEffect removes a podman volume.
type RemoveVolumeEffect struct {
	conn EngineConnection
	Name string
}

func NewRemoveVolumeEffect(conn EngineConnection, name string) effects.Effect {
	return RemoveVolumeEffect{conn: conn, Name: name}
}

func (e RemoveVolumeEffect) Apply() error {
	exists, err := volumes.Exists(e.conn, e.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to check if volume %q exists: %w", e.Name, err)
	}
	if !exists {
		return nil
	}
	err = volumes.Remove(e.conn, e.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to remove volume %q: %w", e.Name, err)
	}
	return nil
}

func ptrBool(b bool) *bool {
	return &b
}
