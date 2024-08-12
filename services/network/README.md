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

## Module architecture
It is divided into 2 parts:
- `client`, part of cli app running on local
- `server`, part of server app running on remote machine

Each part follows hexagonal architecture. It divides into following layers:
- `application`, contains the application logic
- `infrastructure`, contains the implementation of the ports and adapters
  - `input`, contains user integration adapters implementation 
  - `output`, contains adapters to external systems implementation

Other:
- [./network.go](./network.go) contains the network related commands
- [./network.proto](./network.proto) protobuf definition of client-server communication
- [./network.pb.go](./network.pb.go) GoLang definition of `./network.proto` file. **Auto-generated**. Do not edit manually
- [./network_grpc.pb.go](./network_grpc.pb.go) Pure definition of GRPC client and server. **Auto-generated**. Do not edit manually
- [./network_grpcproxy.pb.go](./network_grpcproxy.pb.go) Sansshell extension of pure GRPC client and server, go [here](../../proxy/README.md) to know more. **Auto-generated**. Do not edit manually

