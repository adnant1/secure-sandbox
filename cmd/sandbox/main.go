package main

import (
	"os"
	"sandbox-runtime/internal/cli"
	"sandbox-runtime/internal/config"
	"sandbox-runtime/internal/manager"
	"sandbox-runtime/internal/state"
)

// Main entrypoint into the runtime
func main() {
	cfg := config.Config{
		RootDir: "./sandbox-data",
	}
	store := state.New()
	mgr := manager.New(store, cfg)
	cli := cli.New(mgr)
	cli.Run(os.Args[1:])
}
