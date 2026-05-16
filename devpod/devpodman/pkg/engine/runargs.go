package engine

import (
	"github.com/containers/podman/v5/cmd/podman/common"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// ApplyRunArgs parses runArgs from devcontainer.json and applies the
// resulting options to the provided ContainerCreateOptions.
// runArgs take precedence — any flag specified here overrides what was
// already set on opts.
func ApplyRunArgs(opts *entities.ContainerCreateOptions, runArgs []string) error {

	cmd := &cobra.Command{
		SilenceUsage: true,
	}

	common.DefineCreateFlags(cmd, opts, entities.CreateMode)
	common.DefineNetFlags(cmd)

	if err := cmd.ParseFlags(runArgs); err != nil {
		return err
	}

	// DefineCreateFlags does not bind all flags to the struct.
	// Extract unbound flags manually after parsing.
	if cmd.Flags().Changed("entrypoint") {
		val, err := cmd.Flags().GetString("entrypoint")
		if err != nil {
			return err
		}
		opts.Entrypoint = &val
	}
	if cmd.Flags().Changed("env") || cmd.Flags().Changed("e") {
		vals, err := cmd.Flags().GetStringArray("env")
		if err != nil {
			return err
		}
		opts.Env = vals
	}

	return nil
}
