# HTTPOVERRPC
This directory contains the http over rpc services. It is a simple service that allows to send http requests to from target hosts


## Usage

### sanssh httpoverrpc get
Make a HTTP(-S) request to a specified port on the remote host.

```bash
sanssh <sanssh-args> get [-method <method>] [-header <header>] [-dial-address <dial-address>] [-insecure-skip-tls-verify] [-show-response-headers] [-body <body>] [-protocol <protocol>] [-hostname <hostname>] <remoteport> <request_uri>:
```

Where:
- `<body>` body to send in request
- `<dial-address>` host:port to dial to. If not provided would dial to original host and port
- `<header>` Header to send in the request, may be specified multiple times.
- `<hostname>` ip address or domain name to specify host (default "localhost")
- `<method>` method to use in the HTTP request (default "GET")
- `<protocol>` protocol to communicate with specified hostname (default "http")
- `<remoteport>` port to connect to on the remote host
- `<request_uri>` URI to request

Flags:
- `-show-response-headers` If provided, print response code and headers
- `-insecure-skip-tls-verify` If provided, skip TLS verification


Note:
1. The prefix / in request_uri is always needed, even there is nothing to put
2. If we use `--hostname` to send requests to a specified host instead of the default localhost, and want to use sansshell proxy action
   to proxy requests, don't forget to add `--allow-any-host` for proxy action

Examples:
```bash
# send get request to https://10.1.23.4:9090/hello
httpoverrpc get --hostname 10.1.23.4 --protocol https 9090 /hello
# send get request with url http://example.com:9090/hello, but deal to localhost:9090
httpoverrpc get --hostname example.com --dialAddress localhost:9090 9090 /hello
```
