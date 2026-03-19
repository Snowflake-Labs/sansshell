# mtls testdata

This directory contains test certificates for three independent CAs:

- **Acme Co** (`root.pem`) — client cert `client.pem`, server cert `leaf.pem`
- **Other Corp** (`root2.pem`) — client cert `client2.pem`, server cert `leaf2.pem`
- **Chain Corp** (`root3.pem`) — uses an intermediate CA (`intermediate3.pem`).
  Client cert `client3.pem` and server cert `leaf3.pem` are signed by the
  intermediate, not the root directly. `client3.pem` bundles the leaf +
  intermediate chain. `leaf3_chain.pem` bundles the server leaf + intermediate.
  Used to test chain traversal in `selectIdentity`.
- **Merged roots** (`roots_merged.pem`) — concatenation of all three CA root certs,
  used by multi-identity tests where a single TLS config must trust all CAs.

## Generating private keys

```bash
openssl genrsa -out client.key
openssl genrsa -out leaf.key
openssl genrsa -out root.key

openssl genrsa -out client2.key
openssl genrsa -out leaf2.key
openssl genrsa -out root2.key
```

## Generating certificates

### Acme Co (primary CA)

```bash
openssl req -new -key root.key -out root.csr -subj "/O=Acme Co"
openssl x509 -req -days 30000 -in root.csr -signkey root.key -out root.pem

openssl req -new -key client.key -out client.csr -subj "/O=Acme Co/OU=group1/OU=group2/CN=sanssh"
openssl x509 -req -days 3000 -in client.csr -CA root.pem -CAkey root.key  -out client.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = clientAuth
basicConstraints = critical, CA:FALSE
EOF

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

rm root.csr client.csr leaf.csr
```

### Other Corp (second CA for multi-identity tests)

```bash
openssl req -new -key root2.key -out root2.csr -subj "/O=Other Corp"
openssl x509 -req -days 30000 -in root2.csr -signkey root2.key -out root2.pem

openssl req -new -key client2.key -out client2.csr -subj "/O=Other Corp/CN=other-client"
openssl x509 -req -days 3000 -in client2.csr -CA root2.pem -CAkey root2.key -out client2.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = clientAuth
basicConstraints = critical, CA:FALSE
EOF

openssl req -new -key leaf2.key -out leaf2.csr -subj "/O=Other Corp/CN=other-server"
openssl x509 -req -days 3000 -in leaf2.csr -CA root2.pem -CAkey root2.key -out leaf2.pem -extensions req_ext -extfile /dev/stdin <<EOF
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

rm root2.csr client2.csr leaf2.csr
```

### Chain Corp (intermediate CA for chain traversal tests)

```bash
openssl req -new -x509 -key root3.key -out root3.pem -days 30000 -subj "/O=Chain Corp/CN=Chain Root CA"

openssl req -new -key intermediate3.key -out intermediate3.csr -subj "/O=Chain Corp/CN=Chain Intermediate CA"
openssl x509 -req -days 30000 -in intermediate3.csr -CA root3.pem -CAkey root3.key -CAcreateserial \
  -out intermediate3.pem -extensions v3_ca -extfile /dev/stdin <<EOF
[v3_ca]
basicConstraints = critical, CA:TRUE, pathlen:0
keyUsage = critical, keyCertSign, cRLSign
EOF

openssl req -new -key client3.key -out client3.csr -subj "/O=Chain Corp/CN=chain-client"
openssl x509 -req -days 3000 -in client3.csr -CA intermediate3.pem -CAkey intermediate3.key \
  -out client3_leaf.pem -extensions req_ext -extfile /dev/stdin <<EOF
[req_ext]
keyUsage = critical, digitalSignature
extendedKeyUsage = clientAuth
basicConstraints = critical, CA:FALSE
EOF
cat client3_leaf.pem intermediate3.pem > client3.pem

openssl req -new -key leaf3.key -out leaf3.csr -subj "/O=Chain Corp/CN=chain-server"
openssl x509 -req -days 3000 -in leaf3.csr -CA intermediate3.pem -CAkey intermediate3.key \
  -out leaf3.pem -extensions req_ext -extfile /dev/stdin <<EOF
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
cat leaf3.pem intermediate3.pem > leaf3_chain.pem

rm intermediate3.csr client3.csr client3_leaf.pem leaf3.csr root3.srl intermediate3.srl
```

### Merged roots

```bash
cat root.pem root2.pem root3.pem > roots_merged.pem
```

## Viewing contents

```bash
openssl x509 -in client.pem -text -noout
openssl x509 -in leaf.pem -text -noout
openssl x509 -in root.pem -text -noout
openssl x509 -in client2.pem -text -noout
openssl x509 -in leaf2.pem -text -noout
openssl x509 -in root2.pem -text -noout
```
