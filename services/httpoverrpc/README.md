# HTTPOVERRPC
This directory contains the http over rpc services. It is a simple service that allows to send http requests to from target hosts


## Usage

### sanssh httpoverrpc get
Make a HTTP(-S) request to a specified port on the remote host.

```bash
sanssh <sanssh-args> get [-method <method>] [-header <header>] [-dialAddress <dial-address>] [-insecure-skip-tls-verify] [-show-response-headers] [-body <body>] [-protocol <protocol>] [-hostname <hostname>] <remoteport> <request_uri>:
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
