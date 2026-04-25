package main

import (
	"context"
	"log"
	"os"

	"github.com/niule-eu/devpodman/internal/cli"
	urfave "github.com/urfave/cli/v3"
)

func main() {
	app := &urfave.Command{
		Name:  "podmandev",
		Usage: "devcontainers for podman",
		Commands: []*urfave.Command{
			cli.NewDebugCommand(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
