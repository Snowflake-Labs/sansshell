# mtls testdata

This directory contains some test certificates.

If you need to regenerate them, here's some helpful commands. Openssl

## Generating private keys

These commands will generate new private keys.

```bash
openssl genrsa -out client.key
openssl genrsa -out leaf.key
openssl genrsa -out root.key
```

## Generating certificates

These commands will generate new certificates. We need to generate certificate signing requests as part of generating certificates, but we can delete them afterwards.

```bash
# CA cert
openssl req -new -key root.key -out root.csr -subj "/O=Acme Co"
openssl x509 -req -days 30000 -in root.csr -signkey root.key -out root.pem

# Cert for the client
openssl req -new -key client.key -out client.csr -subj "/O=Acme Co/OU=group1/OU=group2/CN=sanssh"
openssl x509 -req -days 3000 -in client.csr -CA root.pem -CAkey root.key  -out client.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = clientAuth
basicConstraints = critical, CA:FALSE
EOF

# Cert for the server and/or proxy
openssl req -new -key leaf.key -out leaf.csr -subj "/O=Acme Co/OU=group2/OU=group3/CN=sansshell-server"
openssl x509 -req -days 3000 -in leaf.csr -CA root.pem -CAkey root.key -out leaf.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = serverAuth
basicConstraints = critical, CA:FALSE
subjectAltName = @alt_names

[alt_names]
DNS.0 = localhost
DNS.1 = bufnet
IP.0 = 127.0.0.1
IP.1 = ::1
EOF

# Cleanup
rm root.csr client.csr leaf.csr
```

## Viewing contents

These commands will print out the information embedded in the certificates.

```bash
openssl x509 -in client.pem -text -noout
openssl x509 -in leaf.pem -text -noout
openssl x509 -in root.pem -text -noout
```
