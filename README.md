# Unshelled
A non-interactive daemon for host management

Unshelled is primarily gRPC server with a variety of options for localhost
debugging and management. Its goal is to replace the need to use an
interactive shell for emergency debugging and recovery with a much safer
interface. Each authorized action can be evaluated against an OPA policy,
audited in advance or after the fact, and ideally deterministic (for a given
state of the local machine).

unshelled-client is a simple CLI with a friendly API for dumping debugging
state and interacting with a remote machine.  It also includes a set of
convenint but perhaps-less-friendly subcommands to address the raw Unshelled
API endpoints.

# Getting Started
TODO: demonstrate building

# A tour of the codebase
Unshelled is composed of 3 primary concepts:
   1. A series of services, which live in the =services/= directory.
   1. A server which wraps these services into a local host agent.
   1. A reference server binary, which includes all of the services.
   1. A CLI, which serves as the reference implementation of how to use the
      services via the agent.

## Services
Services implement at least one gRPC API endpoint, and expose it by calling
=RegisterUnshelledService= from =init()=.  The goal is to allow custom
implementations of the Unshelled Server to easily import services they wish to
use, and have zero overhead or risk from services they do not import at compile
time.

TODO: Document service/.../client expectations.

## The Server class
Most of the logic of instantiating a local Unshelled server lives in =/server=.
This instantiates a gRPC server, registers the imported services with that
server, and constraints them with the supplied OPA policy.

## The reference Server binary
There is a reference implementation of an Unshelled Server in
=cmd/unshelled-server=, which should be suitable as-written for most use cases.
It's intentionally kept relatively short, so that it can be copied to another
repository and customized by adjusting only the imported services.

## The reference CLI client
There is a referenc implementation of an Unshelled CLI Client in
=cmd/unshelled-client=.  It provides raw access to each gRPC endpoint, as well
as a way to implement "convenience" commands which chain together a series of
actions.

# Extending Unshelled
The server and client are modular, and you should be able to copy a very
minimal set of the code (a handful of lines) and make your own derivative of
Unshelled with more (or less!) functionality.

TODO: Add example client and server, building in different unshelled modules.
