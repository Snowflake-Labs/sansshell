# Network
This directory contains the network services. This defines network related commands like `ping`, `traceroute`, `netstat`, etc.

What is not part of network service? Commands related to protocols build on top of network layer like `http`, `ftp`, `ssh`, etc.

## Usage

### sanssh network tcp-check
Check if a TCP port is open on a remote host.

```bash
sanssh <sanssh-args> network tcp-check <host>:<port> [--timeout <timeout>] [--source-port <source-port>]
```
Where:
- `<sanssh-args>` common sanssh arguments
- `<host>` is the host to check
- `<port>` is the port to check
- `<timeout>` timeout in seconds to wait for the connection to be established. Default is 3 seconds.
- `<source-port>` local source port to bind for the outgoing TCP connection (1-65535). If unset, the OS
  chooses an ephemeral port. Binding privileged ports (`< 1024`) requires the sansshell-server process
  to have the necessary host privilege (e.g. root or `CAP_NET_BIND_SERVICE`).

# Useful docs
- [service-architecture](../../docs/services-architecture.md)
