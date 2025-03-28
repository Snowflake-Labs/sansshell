# SansShell

[![Build Status](https://github.com/Snowflake-Labs/sansshell/workflows/Build%20and%20Test/badge.svg?branch=main)](https://github.com/Snowflake-Labs/sansshell/actions?query=workflow%3A%22Build+and+Test%22)
[![License](https://img.shields.io/:license-Apache%202-brightgreen.svg)](https://github.com/Snowflake-Labs/sansshell/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/Snowflake-Labs/sansshell.svg)](https://pkg.go.dev/github.com/Snowflake-Labs/sansshell)
[![Report Card](https://goreportcard.com/badge/github.com/Snowflake-Labs/sansshell)](https://goreportcard.com/report/github.com/Snowflake-Labs/sansshell)

A non-interactive daemon for host management

```mermaid
flowchart LR;

subgraph sanssh ["sansshell client (sanssh)"]
    cli;
    client;
    subgraph client modules
      package([package]);
      file([file]);
      exec([exec]);
    end
    cli --> package --> client;
    cli --> file --> client;
    cli --> exec --> client;
end
subgraph proxy ["proxy (optional)"]
    proxy_server[proxy-server];
    opa_policy[(opa policy)];
    proxy_server --> opa_policy --> proxy_server
end
subgraph sansshell server ["sansshell server (on each host)"]
    server[sansshell-server];
    host_apis;
    s_opa_policy[(opa policy)];
    subgraph service modules
      s_package([package]);
      s_file([file]);
      s_exec([exec]);
    end
    server --> s_package --> host_apis;
    server --> s_file --> host_apis;
    server --> s_exec --> host_apis;
    server --> s_opa_policy --> server
end
user{user};
user --> cli;
client --"gRPC (mTLS)"--> proxy_server
proxy_server --"grpc (mTLS)"---> server
```

SansShell is primarily a gRPC server with a variety of options for localhost
debugging and management. Its goal is to replace the need to use an
interactive shell for emergency debugging and recovery with a much safer
interface. Each authorized action can be evaluated against an OPA policy,
audited in advance or after the fact, and is ideally deterministic (for a given
state of the local machine).

sanssh is a simple CLI with a friendly API for dumping debugging state and
interacting with a remote machine. It also includes a set of convenient but
perhaps-less-friendly subcommands to address the raw SansShell API endpoints.

## Getting Started

How to set up, build and run locally for testing. All commands are relative to
the project root directory.

Building SansShell requires a recent version of Go (check the go.mod file for
the current version).

### Build and run

You need to populate ~/.sansshell with certificates before running.

```
$ cp -r auth/mtls/testdata ~/.sansshell
```

Then you can build and run the server, in separate terminal windows:

```
$ go run ./cmd/sansshell-server
$ go run ./cmd/sanssh --targets=localhost file read /etc/hosts
```

You can also run the proxy to try the full flow:

```
$ go run ./cmd/sansshell-server
$ go run ./cmd/proxy-server
$ go run ./cmd/sanssh --proxy=localhost:50043 --targets=localhost:50042 file read /etc/hosts
```

Minimal debugging UIs are available at http://localhost:50044 for the server and http://localhost:50046 for the proxy by default.

### Environment setup : protoc

When making any change to the protocol buffers, you'll also need the protocol
buffer compiler (`protoc`) (version 3 or above) as well as the protoc plugins
for Go and Go-GRPC

On MacOS, the protocol buffer can be installed via homebrew using

```
brew install protobuf
```

On Linux, protoc can be installed using either the OS package manager, or by
directly installing a release version from the [protocol buffers github][1]

### Environment setup : protoc plugins

On any platform, once protoc has been installed, you can install the required
code generation plugins using `go install`.

```
$ go install google.golang.org/protobuf/cmd/protoc-gen-go
$ go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
$ go install github.com/Snowflake-Labs/sansshell/proxy/protoc-gen-go-grpcproxy
```

Note that, you'll need to make certain that your `PATH` includes the gobinary
directory (either the value of `$GOBIN`, or, if unset, `$HOME/go/bin`)

The `tools.go` file contains helpful `go generate` directives which will
do this for you, as well as re-generating the service proto files.

```
$ go generate tools.go
```

### Dev Environment setup
#### Required tools
- [pre-commit 3.8.0+](https://pre-commit.com/index.html)
- [golangci-lint 1.59.1+](https://golangci-lint.run/welcome/install/#local-installation)

Configuration:
- Set up git pre-commit hooks
```bash
pre-commit install
```

### Creating your own certificates

As an alternative to copying auth/mtls/testdata, you can create your own example mTLS certs. See the
[mtls testdata readme](/auth/mtls/testdata/README.md) for steps.

### Debugging

Reflection is included in the RPC servers (proxy and sansshell-server)
allowing for the use of [grpc_cli](https://github.com/grpc/grpc/blob/master/doc/command_line_tool.md).

If you are using the certificates from above in ~/.sansshell invoking
grpc_cli requires some additional flags for local testing:

```
$ GRPC_DEFAULT_SSL_ROOTS_FILE_PATH=$HOME/.sansshell/root.pem grpc_cli \
  --ssl_client_key=$HOME/.sansshell/client.key --ssl_client_cert=$HOME/.sansshell/client.pem \
  --ssl_target=127.0.0.1 --channel_creds_type=ssl ls 127.0.0.1:50043
```

NOTE: This connects to the proxy. Change to 50042 if you want to connect to the sansshell-server.

### Testing
To run unit tests, run the following command:
```bash
go test ./...
```

To run integration tests, run the following command:
```bash
# Run go integration tests
INTEGRATION_TEST=yes go test -run "^TestIntegration.*$" ./...

# Run bash integration tests
./test/integration.sh
```

#### Integration testing
To implement integration tests, you need to:
- Create a new test file name satisfy pattern `<file-name>_integration_test.go`
- Name test functions satisfy pattern `TestIntegration<FunctionName>`
- Add check to skip tests when unit test is running:
```go
if os.Getenv("INTEGRATION_TEST") == "" {
    t.Skip("skipping integration test")
}
```

## A tour of the codebase

SansShell is composed of 5 primary concepts:

1.  A series of services, which live in the `services/` directory.
1.  A server which wraps these services into a local host agent.
1.  A proxy server which can be used as an entry point to processing sansshell
    RPCs by validating policy and then doing fanout to 1..N actual
    sansshell servers. This can be done as a one to many RPC where
    a single incoming RPC is replicated to N backend hosts in one RPC call.
1.  A reference server binary, which includes all of the services.
1.  A CLI, which serves as the reference implementation of how to use the
    services via the agent.

### Services

Services implement at least one gRPC API endpoint, and expose it by calling
`RegisterSansShellService` from `init()`. The goal is to allow custom
implementations of the SansShell Server to easily import services they wish to
use, and have zero overhead or risk from services they do not import at compile
time.

[Here](/docs/services-architecture.md) you could read more about services architecture.

#### List of available Services

1. Ansible: Run a local ansible playbook and return output
1. Execute: Execute a command
1. HealthCheck
1. File operations: Read, Write, Stat, Sum, rm/rmdir, chmod/chown/chgrp
   and immutable operations (if OS supported).
1. Package operations: Install, Upgrade, List, Repolist
1. Process operations: List, Get stacks (native or Java), Get dumps (core or Java heap)
1. MPA operations: Multi party authorization for commands
1. [Network](./services/network):
   1. [TCP-Check](./services/network/README.md#sanssh-network-tcp-check) - Check if a TCP port is open on a remote host
1. Service operations: List, Status, Start/stop/restart

TODO: Document service/.../client expectations.

#### Services API versioning and OPA policy

In most cases, services APIs evolve in a fully-backward compatible model,
where adding new parameters or behaviors do not cause unintentional
side-effects on authz decisions made by OPA policy.

Now consider a localfile read which accepts a path to a file, for example
`/tmp/test.txt`. If we extend this service to allow reading all files
in a particular directory (through read request with `/tmp/*` as argument)
we may end up allowing to read `/tmp/secret` file which could be explicitly
denied in the OPA policy.

To allow extensions of Sansshell services functions in a safe way we introduced
a notion of `API version` which follows https://semver.org/. A MAJOR version will
be changed each time we add a backward-incompatible change to Sansshell services.

Default version supported by Sanasshell server is set to `1.0.0`, in order to use
features of higher API version you should audit your OPA policy to check if there
are no unintentional side-effects of allowing new Sansshell features.

#### List of current API versions

  - `1.0.0` -- current snapshot of Sansshell API as of
        https://github.com/Snowflake-Labs/sansshell/tree/v1.40.4.
  - `2.0.0` -- allow to read a contents of whole directory  by specifying a trailing
        wildcard, for example `localfile read /tmp/*`.

### The Server class

Most of the logic of instantiating a local SansShell server lives in the
`server` directory. This instantiates a gRPC server, registers the imported
services with that server, and constraints them with the supplied OPA policy.

### The reference Proxy Server binary

There is a reference implementation of a SansShell Proxy Server in
`cmd/proxy-server`, which should be suitable as-written for many use cases.
It's intentionally kept relatively short, so that it can be copied to another
repository and customized by adjusting only the imported services.

### The reference Server binary

There is a reference implementation of a SansShell Server in
`cmd/sansshell-server`, which should be suitable as-written for some use cases.
It's intentionally kept relatively short, so that it can be copied to another
repository and customized by adjusting only the imported services.

### The reference CLI client

There is a reference implementation of a SansShell CLI Client in
`cmd/sanssh`. It provides raw access to each gRPC endpoint, as well
as a way to implement "convenience" commands which chain together a series of
actions.

It also demonstrates how to set up command line completion. To use this, set
the appropriate line in your shell configuration.

```shell
# In .bashrc
complete -C /path/to/sanssh -o dirnames sanssh
# Or in .zshrc
autoload -Uz compinit && compinit
autoload -U +X bashcompinit && bashcompinit
complete -C /path/to/sanssh -o dirnames sanssh
```

## Multi party authorization

MPA, or [multi party authorization](https://en.wikipedia.org/wiki/Multi-party_authorization),
allows guarding sensitive commands behind additional approval. SansShell
supports writing authorization policies that only pass when a command is
approved by additional entities beyond the caller. See
[services/mpa/README.md](/services/mpa/README.md) for details on
implementation and usage.

To try this out in the reference client, run the following commands in parallel
in separate terminals. This will run a server that accepts any command from a
proxy and a proxy that allows MPA requests from the "sanssh" user when approved by the "approver" user.

```bash
# Start the server
go run ./cmd/sansshell-server -server-cert ./auth/mtls/testdata/leaf.pem -server-key ./auth/mtls/testdata/leaf.key
# Start the proxy
go run ./cmd/proxy-server -client-cert ./services/mpa/testdata/proxy.pem -client-key ./services/mpa/testdata/proxy.key -server-cert ./services/mpa/testdata/proxy.pem -server-key ./services/mpa/testdata/proxy.key
# Run a command gated on MPA
go run ./cmd/sanssh -client-cert ./auth/mtls/testdata/client.pem -client-key ./auth/mtls/testdata/client.key -mpa -proxy localhost -targets localhost exec run /bin/echo hello world
# Approve the command above
go run ./cmd/sanssh -client-cert ./services/mpa/testdata/approver.pem -client-key ./services/mpa/testdata/approver.key -proxy localhost -targets localhost mpa approve 53feec22-5447f403-c0e0a419
```

## Extending SansShell

SansShell is built on a principle of "Don't pay for what you don't use". This
is advantageous in both minimizing the resources of SansShell server (binary
size, memory footprint, etc) as well as reducing the security risk of running
it. To accomplish that, all of the SansShell services are independent modules,
which can be optionally included at build time. The reference server and
client provide access to the features of all of the built-in modules, and come
with exposure to all of their potential bugs and bloat.

As a result, we expect most users of SansShell would want to copy a very
minimal set of the code (a handful of lines from the reference client and
server), import only the modules they intend to use, and build their own
derivative of SansShell with more (or less!) functionality.

That same extensibility makes it easy to add additional functionality by
implementing your own module.

To quickly rebuild all binaries you can run:

```
$ go generate build.go
```

and they will be placed in a bin directory (which is ignored by git).

TODO: Add example client and server, building in different SansShell modules.

If you need to edit a proto file (to augment an existing service or
create a new one) you'll need to generate proto outputs.

```
$ go generate tools.go
```

NOTE: tools.go will need to have additions to it if you add new services.

[1]: https://github.com/protocolbuffers/protobuf/releases
