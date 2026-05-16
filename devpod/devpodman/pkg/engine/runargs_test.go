package engine

import (
	"testing"

	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/niule-eu/devpodman/pkg/devcontainer"
	"github.com/niule-eu/devpodman/pkg/model"
)

func TestApplyRunArgs(t *testing.T) {
	tests := []struct {
		name    string
		runArgs []string
		check   func(t *testing.T, opts *entities.ContainerCreateOptions)
	}{
		{
			name:    "empty",
			runArgs: []string{},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.Entrypoint != nil && *opts.Entrypoint != "" {
					t.Errorf("expected empty entrypoint, got %q", *opts.Entrypoint)
				}
			},
		},
		{
			name:    "entrypoint string",
			runArgs: []string{"--entrypoint", "/bin/bash"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.Entrypoint == nil || *opts.Entrypoint != "/bin/bash" {
					t.Errorf("expected entrypoint \"/bin/bash\", got %v", opts.Entrypoint)
				}
			},
		},
		{
			name:    "entrypoint json",
			runArgs: []string{"--entrypoint", "[\"/bin/bash\",\"-l\"]"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.Entrypoint == nil || *opts.Entrypoint != "[\"/bin/bash\",\"-l\"]" {
					t.Errorf("expected entrypoint json, got %q", *opts.Entrypoint)
				}
			},
		},
		{
			name:    "env",
			runArgs: []string{"--env", "FOO=bar"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if len(opts.Env) != 1 || opts.Env[0] != "FOO=bar" {
					t.Errorf("expected env [FOO=bar], got %v", opts.Env)
				}
			},
		},
		{
			name:    "user",
			runArgs: []string{"--user", "1000:1000"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.User != "1000:1000" {
					t.Errorf("expected user \"1000:1000\", got %q", opts.User)
				}
			},
		},
		{
			name:    "workdir",
			runArgs: []string{"--workdir", "/app"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.Workdir != "/app" {
					t.Errorf("expected workdir \"/app\", got %q", opts.Workdir)
				}
			},
		},
		{
			name:    "hostname",
			runArgs: []string{"--hostname", "myhost"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.Hostname != "myhost" {
					t.Errorf("expected hostname \"myhost\", got %q", opts.Hostname)
				}
			},
		},
		{
			name:    "privileged",
			runArgs: []string{"--privileged"},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if !opts.Privileged {
					t.Error("expected privileged to be true")
				}
			},
		},
		{
			name:    "healthlogdestination",
			runArgs: []string{""},
			check: func(t *testing.T, opts *entities.ContainerCreateOptions) {
				if opts.HealthLogDestination != "local" {
					t.Error("expected HealthLogDestination to be default of 'local'")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &entities.ContainerCreateOptions{}
			common.DefineCreateDefaults(opts)
			err := ApplyRunArgs(opts, tt.runArgs)
			if err != nil {
				t.Fatalf("ApplyRunArgs failed: %v", err)
			}
			tt.check(t, opts)
		})
	}
}

func TestBuildMainContainerSpec_RunArgsReflectedInSpec(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *devcontainer.ResolvedConfig
		check   func(t *testing.T, spec *specgen.SpecGenerator)
	}{
		{
			name: "runArgs entrypoint overrides image entrypoint",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name: "test-runargs",
				},
				NonCompose: &model.NonComposeBase{
					RunArgs: []string{"--entrypoint", "/bin/sh"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if len(spec.Entrypoint) != 1 || spec.Entrypoint[0] != "/bin/sh" {
					t.Errorf("expected entrypoint [/bin/sh], got %v", spec.Entrypoint)
				}
			},
		},
		{
			name: "runArgs user overrides remoteUser",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name:       "test-runargs",
					RemoteUser: "nobody",
				},
				NonCompose: &model.NonComposeBase{
					RunArgs: []string{"--user", "1000:1000"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if spec.User != "1000:1000" {
					t.Errorf("expected user \"1000:1000\", got %q", spec.User)
				}
			},
		},
		{
			name: "runArgs workdir overrides workspaceFolder",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name: "test-runargs",
				},
				NonCompose: &model.NonComposeBase{
					WorkspaceFolder: "/workspace",
					RunArgs:         []string{"--workdir", "/app"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if spec.WorkDir != "/app" {
					t.Errorf("expected workdir \"/app\", got %q", spec.WorkDir)
				}
			},
		},
		{
			name: "runArgs env overrides devcontainer env",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name:      "test-runargs",
					RemoteEnv: map[string]string{"MY_VAR": "from-config"},
				},
				NonCompose: &model.NonComposeBase{
					ContainerEnv: map[string]string{"CT_VAR": "from-config"},
					RunArgs:      []string{"--env", "OVERRIDE=yes"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if spec.Env["OVERRIDE"] != "yes" {
					t.Errorf("expected env OVERRIDE=yes, got %v", spec.Env)
				}
			},
		},
		{
			name: "runArgs hostname is reflected",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name: "test-runargs",
				},
				NonCompose: &model.NonComposeBase{
					RunArgs: []string{"--hostname", "myhost"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if spec.Hostname != "myhost" {
					t.Errorf("expected hostname \"myhost\", got %q", spec.Hostname)
				}
			},
		},
		{
			name: "runArgs privileged is reflected",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name: "test-runargs",
				},
				NonCompose: &model.NonComposeBase{
					RunArgs: []string{"--privileged"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if !spec.IsPrivileged() {
					t.Error("expected privileged to be true")
				}
			},
		},
		{
			name: "HealthLogDestination is local",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name: "test-runargs",
				},
				NonCompose: &model.NonComposeBase{
					RunArgs: []string{"--privileged"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if spec.HealthLogDestination != "local" {
					t.Error("expected privileged to be true")
				}
			},
		},
		{
			name: "no runArgs uses config values",
			cfg: &devcontainer.ResolvedConfig{
				Image: &model.ImageContainer{Image: "alpine:latest"},
				Common: &model.DevContainerCommon{
					Name:       "test-runargs",
					RemoteUser: "root",
					RemoteEnv:  map[string]string{"FOO": "bar"},
				},
				NonCompose: &model.NonComposeBase{
					WorkspaceFolder: "/workspace",
					ContainerEnv:    map[string]string{"BAZ": "qux"},
				},
			},
			check: func(t *testing.T, spec *specgen.SpecGenerator) {
				if spec.User != "root" {
					t.Errorf("expected user \"root\", got %q", spec.User)
				}
				if spec.WorkDir != "/workspace" {
					t.Errorf("expected workdir \"/workspace\", got %q", spec.WorkDir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := buildMainContainerSpec(tt.cfg, "/tmp/test")
			if err != nil {
				t.Fatalf("buildMainContainerSpec failed: %v", err)
			}
			tt.check(t, spec)
		})
	}
}
