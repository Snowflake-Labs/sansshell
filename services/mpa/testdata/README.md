This directory contains additional certificates for testing MPA. See [/auth/mtls/testdata/](/auth/mtls/testdata/) for more info on these.

```
openssl genrsa -out approver.key
openssl req -new -key approver.key -out approver.csr -subj "/O=Acme Co/OU=group1/OU=group2/CN=approver"
openssl x509 -req -days 3000 -in approver.csr -CA ../../../auth/mtls/testdata/root.pem -CAkey ../../../auth/mtls/testdata/root.key  -out approver.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = clientAuth
basicConstraints = critical, CA:FALSE
EOF
rm approver.csr
```

```
openssl genrsa -out proxy.key
openssl req -new -key proxy.key -out proxy.csr -subj "/O=Acme Co/CN=proxy"
openssl x509 -req -days 3000 -in proxy.csr -CA ../../../auth/mtls/testdata/root.pem -CAkey ../../../auth/mtls/testdata/root.key  -out proxy.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = serverAuth, clientAuth
basicConstraints = critical, CA:FALSE
subjectAltName = @alt_names

[alt_names]
DNS.0 = localhost
DNS.1 = bufnet
IP.0 = 127.0.0.1
IP.1 = ::1
EOF
rm proxy.csr
```
