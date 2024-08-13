## Services architecture
SansShell is built on a principle of "Don't pay for what you don't use". This
is advantageous in both minimizing the resources of SansShell server (binary
size, memory footprint, etc) as well as reducing the security risk of running
it. To accomplish that, all of the SansShell services are independent modules,
which can be optionally included at build time.

It is divided into 2 parts:
- `client`, part of cli app running on local
- `server`, part of server app running on remote machine

Each part follows hexagonal architecture. It divides into following layers:
- `application`, contains the application logic
- `infrastructure`, contains the implementation of the ports and adapters
    - `input`, contains user interface/api adapters implementation, such as GRPC controllers, CLI command handlers and etc
    - `output`, contains adapters to external systems implementation, such as HTTP/GRPC client, repositories and etc

Other:
- `./<service-name>.go` contains the service related commands
- `./<service-name>.proto` protobuf definition of client-server communication
- `./<service-name>.pb.go` GoLang definition of `./<service-name>.proto` file. **Auto-generated**. Do not edit manually
- `./<service-name>_grpc.pb.go` Pure definition of GRPC client and server. **Auto-generated**. Do not edit manually
- `./<service-name>_grpcproxy.pb.go` Sansshell extension of pure GRPC client and server, go [here](../proxy/README.md) to know more. **Auto-generated**. Do not edit manually
