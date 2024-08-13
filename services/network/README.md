# Network
This directory contains the network services. This defines network related commands like `ping`, `traceroute`, `netstat`, etc.

What is not part of network service? Commands related to protocols build on top of network layer like `http`, `ftp`, `ssh`, etc.

## Commands

### sanssh network tcp-check
Check if a TCP port is open on a remote host.

```bash
sanssh --targets <remote-machine-hosts> network tcp-check --host <host> --port <port>
```
Where:
- `<remote-machine-hosts>` is a list of remote machine hosts
- `<host>` is the host to check
- `<port>` is the port to check

# Useful docs
- [service-architecture](../../docs/services-architecture.md)
