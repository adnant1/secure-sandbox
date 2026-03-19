package cli

import (
	"fmt"
	"sandbox-runtime/internal/manager"
)

func printUsage() {
	fmt.Println("Usage: sandbox <command> [args]")
	fmt.Println("Commands: run, list, inspect, stop")
}

// CLI represents the command-line interface for interaction with the runtime.
type CLI struct {
	mgr *manager.Manager
}

func New(mgr *manager.Manager) *CLI {
	if mgr == nil {
		panic("cli: nil manager")
	}
	return &CLI{
		mgr: mgr,
	}
}

// Run parses command-line arguments and dispatches the appropriate command.
func (c *CLI) Run(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	cmd := args[0]
	switch cmd {
	case "run":
		c.runCommand(args[1:])
	case "list":
		c.listCommand(args[1:])
	case "inspect":
		c.inspectCommand(args[1:])
	case "stop":
		c.stopCommand(args[1:])
	default:
		fmt.Println("Unknown command:", cmd)
		printUsage()
	}
}

func (c *CLI) runCommand(args []string) {
	if len(args) < 2 {
		fmt.Println("usage: sandbox run <bundlePath> <command> [args...]")
		return
	}

	bundlePath := args[0]
	cmd := args[1]
	cmdArgs := args[2:]
	sb, err := c.mgr.CreateSandbox(manager.CreateSandboxRequest{
		BundlePath: bundlePath,
		Command:    cmd,
		Args:       cmdArgs,
	})
	if err != nil {
		fmt.Println("error creating sandbox:", err)
		return
	}

	sb, err = c.mgr.StartSandbox(sb.ID)
	if err != nil {
		fmt.Println("error starting sandbox:", err)
		return
	}
	fmt.Printf("started sandbox: id=%s pid=%d state=%v\n", sb.ID, sb.PID, sb.State)
}

func (c *CLI) listCommand(args []string) {
	sandboxes, err := c.mgr.ListSandboxes()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, sb := range sandboxes {
		fmt.Printf("%s\t%v\n", sb.ID, sb.State)
	}
}

func (c *CLI) inspectCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("missing sandbox id")
		return
	}

	id := args[0]
	sb, err := c.mgr.GetSandbox(id)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("ID: %s\nState: %v\n", sb.ID, sb.State)
}

func (c *CLI) stopCommand(args []string) {
	if len(args) < 1 {
		fmt.Println("missing sandbox id")
		return
	}

	id := args[0]
	sb, err := c.mgr.StopSandbox(id)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("stopped sandbox:", sb.ID)
}
