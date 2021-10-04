# SanShell
A non-interactive daemon for host management

SansShell is primarily gRPC server with a variety of options for localhost
debugging and management. Its goal is to replace the need to use an
interactive shell for emergency debugging and recovery with a much safer
interface. Each authorized action can be evaluated against an OPA policy,
audited in advance or after the fact, and is ideally deterministic (for a given
state of the local machine).

sanssh is a simple CLI with a friendly API for dumping debugging state and
interacting with a remote machine.  It also includes a set of convenient but
perhaps-less-friendly subcommands to address the raw SansShell API endpoints.

# Getting Started
How to setup, build and run locally for testing.  All commands are relative to
the project root directory.

## Environment setup Mac
You need the Go setup as well as the proto compiler.:

```
$ brew install go
$ brew install protoc-gen-go
$ brew install protoc-gen-go-grpc
```

## Environment setup Linux
TODO:

## Build and run
You only need to do these steps once to configure example mTLS certs:
```
$ go get -u github.com/meterup/generate-cert
$ mkdir -m 0700 certs
$ cd certs
$ $(go env GOPATH)/bin/generate-cert --host=localhost,127.0.0.1,::1
$ cd ../
$ ln -s $(pwd)/certs ~/.sansshell
```

Then you can build and run the server, in separate terminal windows:
```
$ cd cmd/sansshell-server && go build && ./sansshell-server
$ cd cmd/sanssh && go build && ./sanssh read /etc/hosts
```

# A tour of the codebase
SansShell is composed of 4 primary concepts:
   1. A series of services, which live in the =services/= directory.
   1. A server which wraps these services into a local host agent.
   1. A reference server binary, which includes all of the services.
   1. A CLI, which serves as the reference implementation of how to use the
      services via the agent.

## Services
Services implement at least one gRPC API endpoint, and expose it by calling
=RegisterSansShellService= from =init()=.  The goal is to allow custom
implementations of the SansShell Server to easily import services they wish to
use, and have zero overhead or risk from services they do not import at compile
time.

###List of available Services:
1) HealthCheck
2) Read File: Read contents of file
3) Execute: Execute a command

TODO: Document service/.../client expectations.

## The Server class
Most of the logic of instantiating a local SansShell server lives in =/server=.
This instantiates a gRPC server, registers the imported services with that
server, and constraints them with the supplied OPA policy.

## The reference Server binary
There is a reference implementation of an SansShell Server in
=cmd/sansshell-server=, which should be suitable as-written for many use cases.
It's intentionally kept relatively short, so that it can be copied to another
repository and customized by adjusting only the imported services.

## The reference CLI client
There is a reference implementation of an SansShell CLI Client in
=cmd/sanssh=.  It provides raw access to each gRPC endpoint, as well
as a way to implement "convenience" commands which chain together a series of
actions.

# Extending SansShell
SansShell is built on a principle of "Don't pay for what you don't use".  This
is advantageous in both minimizing the resources of SansShell server (binary
size, memory footprint, etc) as well as reducing the security risk of running
it.  To accomplish that, all of the SansShell services are independent modules,
which can be optionally included at build time.  The reference server and
client provide access to the features of all of the built-in modules, and come
with exposure to all of their potential bugs and bloat.

As a result, we expect most users of SansShell would want to copy a very
minimal set of the code (a handful of lines from the reference client and
server), import only the modules they intend to use, and build their own
derivative of SansShell with more (or less!) functionality.

That same extensibility makes it easy to add additional functionality by
implementing your own module.

TODO: Add example client and server, building in different SansShell modules.

If you need to edit a proto file (to augment an existing service or 
create a new one) you'll need to generate proto outputs.

```
$ cd services/SERVICE
$ go generate
```
