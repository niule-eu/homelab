package main

import (
	"context"
	"fmt"
	"os"

	"github.com/niule-eu/devpodman/devcontainer"
	"github.com/niule-eu/devpodman/podman"
	"github.com/urfave/cli/v3"
)

func NewDebugCommand() *cli.Command {
	return &cli.Command{
		Name:  "debug",
		Usage: "debug and validate devcontainer configuration",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "validate",
				Aliases: []string{"v"},
			},
			&cli.BoolFlag{
				Name:    "print-podman-config",
				Aliases: []string{"p"},
				Usage:   "load and print podman connection config to stdout",
			},
			&cli.BoolFlag{
				Name:    "list-containers",
				Aliases: []string{"l"},
				Usage:   "list all podman containers",
			},
		},
		Action: debugAction,
	}
}

func debugAction(ctx context.Context, c *cli.Command) error {
	if c.Bool("list-containers") {
		cfg, err := podman.LoadConfig()
		if err != nil {
			return err
		}
		client, err := podman.NewClient(ctx, cfg)
		if err != nil {
			return err
		}
		cts, err := client.ListContainers(ctx)
		if err != nil {
			return err
		}
		for _, ct := range cts {
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", ct.ID, ct.Image, ct.State, ct.Names)
		}
		return nil
	}

	if c.Bool("print-podman-config") {
		cfg, err := podman.LoadConfig()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "socket_path: %s\n", cfg.SocketPath)
		fmt.Fprintf(os.Stdout, "timeout: %s\n", cfg.Timeout)
		fmt.Fprintf(os.Stdout, "connection_uri: %s\n", cfg.ConnectionURI())
		return nil
	}

	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Current working directory: %s\n", dir)

	validatePath := c.String("validate")
	if validatePath == "" {
		return nil
	}

	fileInfo, err := os.Stat(validatePath)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", validatePath, err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", fileInfo)

	data, err := os.ReadFile(validatePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", validatePath, err)
	}

	cfg, err := devcontainer.Load(data)
	if err != nil {
		return err
	}

	if cfg.Build != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded build config: %+v\n", cfg.Build.Build.Args)
	}
	if cfg.Image != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded image config: %s\n", cfg.Image.Image)
	}
	if cfg.Common != nil {
		fmt.Fprintf(os.Stdout, "Successfully loaded common config: %+v\n", cfg.Common)
	}

	return nil
}
