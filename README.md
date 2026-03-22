# Secure Sandbox Runtime

A runtime for secure, isolated workload execution, featuring a custom control plane, Linux namespace isolation, cgroup-based resource enforcement, and syscall-level sandboxing.

---

## Overview

Secure Sandbox Runtime provides a minimal, security-focused execution environment for running workloads in isolated sandboxes on a single host. The system is designed around strict isolation boundaries, explicit control plane ownership, and defense-in-depth mechanisms.

The runtime leverages native Linux primitives including namespaces, cgroups, and seccomp to constrain process behavior and reduce the attack surface of executed workloads.

---

## Architecture

```
sandbox (CLI client)
   ↓
HTTP over Unix domain socket (/run/sandboxd.sock)
   ↓
sandboxd (daemon / control plane)
   ↓
Manager (lifecycle orchestration)
   ↓
init process (re-exec boundary)
   ↓
Linux primitives (namespaces, cgroups, seccomp, mounts)
   ↓
workload
```

---

## Components

### sandboxd (Daemon)

The daemon is the control plane responsible for:

* Managing sandbox lifecycle
* Enforcing isolation boundaries
* Coordinating resource and security constraints
* Exposing a local HTTP API over a Unix domain socket

The daemon is the single authority for all sandbox operations.

---

### sandbox (CLI)

The CLI is a stateless client that:

* Translates user input into API requests
* Communicates with the daemon over a Unix socket
* Displays execution results and sandbox state

---

### Manager

The Manager encapsulates lifecycle operations:

* CreateSandbox
* StartSandbox
* StopSandbox
* GetSandbox
* ListSandboxes

It coordinates between the state store, cgroup subsystem, and execution layer.

---

### Init Process (Re-exec Boundary)

Workloads are executed through a dedicated init process using a re-exec model:

```
sandboxd → exec sandbox init → initproc → workload
```

This ensures that namespace setup, filesystem transitions, and security policies are applied within the correct process context before executing the workload.

---

## Isolation Model

The runtime enforces isolation using native Linux primitives:

### Namespaces

* PID namespace
* Mount namespace
* Extensible to network and user namespaces

### Cgroups

* CPU limits
* Memory limits
* PID limits

### Filesystem

* Root filesystem isolation
* pivot_root for sandboxed execution environment

### Seccomp

* Syscall filtering to restrict kernel surface area
* Default-deny policy with explicit allow rules

---

## API

The daemon exposes a resource-oriented HTTP API over a Unix domain socket.

### Endpoints

```
POST   /sandboxes              Create and start a sandbox
GET    /sandboxes              List sandboxes
GET    /sandboxes/{id}         Inspect sandbox
POST   /sandboxes/{id}/stop    Stop sandbox
POST   /shutdown               Shutdown daemon
```

---

## Build

```
go build -o sandboxd ./cmd/sandboxd
go build -o sandbox  ./cmd/sandbox
```

---

## Execution

### Start daemon

```
sudo ./sandboxd
```

### Run workload

```
sudo ./sandbox run ./bundle <command> [args]
```

### List sandboxes

```
sudo ./sandbox list
```

### Inspect sandbox

```
sudo ./sandbox inspect <id>
```

### Stop sandbox

```
sudo ./sandbox stop <id>
```

### Shutdown daemon

```
sudo ./sandbox shutdown
```

---

## Bundle Format

A bundle defines the filesystem and execution configuration required for a sandbox.

```
bundle/
  rootfs/
  config
```

The `config` file is required and defines the default command, arguments, and resource constraints for the workload. These values may be overridden at runtime via CLI flags.
