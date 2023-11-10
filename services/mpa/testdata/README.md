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
