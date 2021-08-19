package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/snowflakedb/unshelled/services"
)

// Serve wraps up BuildServer in a succinct API for callers
func Serve(hostport string, c credentials.TransportCredentials, policy string) error {
	lis, err := net.Listen("tcp", hostport)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s, err := BuildServer(lis, c, policy)
	if err != nil {
		return err
	}

	return s.Serve(lis)
}

// BuildServer creates a gRPC server, attaches the OPA policy interceptor,
// registers all of the imported Unshelled modules. Separating this from Serve
// primarily facilitates testing.
func BuildServer(lis net.Listener, c credentials.TransportCredentials, policy string) (*grpc.Server, error) {
	o, err := NewOPA(policy)
	if err != nil {
		return &grpc.Server{}, fmt.Errorf("NewOpa: %w", err)
	}
	s := grpc.NewServer(grpc.Creds(c), grpc.UnaryInterceptor(o.Authorize))
	for _, unshelledService := range services.ListServices() {
		unshelledService.Register(s)
	}
	return s, nil
}

// LoadTLSKeys reads the certificates and keys from disk at the supplied paths,
// and assembles them into a set of TransportCredentials for the gRPC server.
func LoadTLSKeys(rootCAFile, clientCertFile, clientKeyFile string) (credentials.TransportCredentials, error) {
	// Read in the root of trust for server identities
	ca, err := ioutil.ReadFile(rootCAFile)
	if err != nil {
		return nil, fmt.Errorf("reading server CA from %q: %v", rootCAFile, err)
	}
	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("adding CA cert: %v", err)
	}

	// Read in client credentials
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading client credentials: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    capool,
	}), nil
}
