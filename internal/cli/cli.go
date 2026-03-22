package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"sandbox-runtime/internal/api"
)

// CLI represents the command-line client for sandboxd.
type CLI struct {
	client     *http.Client
	socketPath string
}

// New initializes a CLI that communicates with sandboxd over a Unix socket.
func New(socketPath string) *CLI {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	client := &http.Client{
		Transport: transport,
	}
	return &CLI{
		client:     client,
		socketPath: socketPath,
	}
}

func printUsage() {
	fmt.Println("Usage: sandbox <command> [args]")
	fmt.Println("Commands: run, list, inspect, stop, shutdown")
}

// Run parses command-line arguments and dispatches commands.
func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmd := args[0]

	switch cmd {
	case "run":
		return c.runCommand(args[1:])
	case "list":
		return c.listCommand()
	case "inspect":
		return c.inspectCommand(args[1:])
	case "stop":
		return c.stopCommand(args[1:])
	case "shutdown":
		return c.shutdownCommand()
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (c *CLI) runCommand(args []string) error {
	req, err := parseRunArgs(args)
	if err != nil {
		fmt.Println("usage: sandbox run <bundlePath> [command] [args...] [--memory=N] [--cpu=N] [--pids=N] [--timeout=N]")
		return nil
	}

	body, _ := json.Marshal(req)

	resp, err := c.client.Post("http://unix/sandboxes", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return errors.New(errResp.Error)
	}

	var sb api.SandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("started sandbox: id=%s pid=%d state=%s\n", sb.ID, sb.PID, sb.State)
	return nil
}

func (c *CLI) listCommand() error {
	resp, err := c.client.Get("http://unix/sandboxes")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return errors.New(errResp.Error)
	}

	var sandboxes []api.SandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&sandboxes); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	for _, sb := range sandboxes {
		fmt.Printf("%s\t%s\n", sb.ID, sb.State)
	}

	return nil
}

func (c *CLI) inspectCommand(args []string) error {
	if len(args) < 1 {
		fmt.Println("usage: sandbox inspect <sandboxID> [--logs]")
		return nil
	}

	id := args[0]

	// --logs flag
	for _, a := range args[1:] {
		if a == "--logs" {
			return c.inspectLogs(id)
		}
	}

	url := fmt.Sprintf("http://unix/sandboxes/%s", id)
	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return errors.New(errResp.Error)
	}

	var sb api.SandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("ID: %s\n", sb.ID)
	fmt.Printf("State: %s\n", sb.State)
	if sb.PID != 0 {
		fmt.Printf("PID: %d\n", sb.PID)
	}
	if sb.StartedAt != "" {
		fmt.Printf("Started At: %s\n", sb.StartedAt)
	}
	if sb.FinishedAt != "" {
		fmt.Printf("Finished At: %s\n", sb.FinishedAt)
	}
	if sb.State == "EXITED" {
		fmt.Printf("Exit Reason: %s\n", sb.ExitReason)
	}
	return nil
}

func (c *CLI) inspectLogs(id string) error {
	url := fmt.Sprintf("http://unix/sandboxes/%s/logs", id)

	resp, err := c.client.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return errors.New(errResp.Error)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read logs: %w", err)
	}

	fmt.Print(string(body))
	return nil
}

func (c *CLI) stopCommand(args []string) error {
	if len(args) < 1 {
		return errors.New("missing sandbox id")
	}

	id := args[0]
	url := fmt.Sprintf("http://unix/sandboxes/%s/stop", id)

	req, _ := http.NewRequest(http.MethodPost, url, nil)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return errors.New(errResp.Error)
	}

	var sb api.SandboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Println("stopped sandbox:", sb.ID)
	return nil
}

func (c *CLI) shutdownCommand() error {
	req, _ := http.NewRequest(http.MethodPost, "http://unix/shutdown", nil)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return errors.New(errResp.Error)
	}

	fmt.Println("sandboxd shutting down...")
	return nil
}

func isFlagKV(a string) bool {
	return strings.HasPrefix(a, "--memory=") ||
		strings.HasPrefix(a, "--cpu=") ||
		strings.HasPrefix(a, "--pids=") ||
		strings.HasPrefix(a, "--timeout=")
}

// parseRunArgs parses CLI input into an API request.
func parseRunArgs(args []string) (api.CreateSandboxRequest, error) {
	if len(args) < 1 {
		return api.CreateSandboxRequest{}, fmt.Errorf("missing bundle path")
	}

	bundlePath := args[0]
	rest := args[1:]

	var workload []string
	var flagArgs []string

	flagStart := -1
	for i, a := range rest {
		if strings.HasPrefix(a, "--") {
			flagStart = i
			break
		}
	}

	if flagStart == 0 && len(rest) > 1 {
		return api.CreateSandboxRequest{}, errors.New("")
	}

	if flagStart == -1 {
		workload = rest
	} else {
		workload = rest[:flagStart]
		flagArgs = rest[flagStart:]
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)

	var mem, cpu, pids, timeout int
	fs.IntVar(&mem, "memory", 0, "")
	fs.IntVar(&cpu, "cpu", 0, "")
	fs.IntVar(&pids, "pids", 0, "")
	fs.IntVar(&timeout, "timeout", 0, "")

	if err := fs.Parse(flagArgs); err != nil {
		return api.CreateSandboxRequest{}, err
	}

	var cmd string
	var cmdArgs []string
	if len(workload) > 0 {
		cmd = workload[0]
		cmdArgs = workload[1:]
	}

	return api.CreateSandboxRequest{
		BundlePath: bundlePath,
		Command:    cmd,
		Args:       cmdArgs,
		Resources: api.ResourceOverrides{
			MemoryMB:   mem,
			CPU:        cpu,
			Pids:       pids,
			TimeoutSec: timeout,
		},
	}, nil
}
