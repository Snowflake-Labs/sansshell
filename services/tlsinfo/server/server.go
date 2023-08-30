/* Copyright (c) 2023 Snowflake Inc. All rights reserved.

   Licensed under the Apache License, Version 2.0 (the
   "License"); you may not use this file except in compliance
   with the License.  You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing,
   software distributed under the License is distributed on an
   "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
   KIND, either express or implied.  See the License for the
   specific language governing permissions and limitations
   under the License.
*/

// Package server implements the server interface for sansshell 'tlsinfo' service.
package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/tlsinfo"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

// Metrics
var (
	tlsDialFailureCounter = metrics.MetricDefinition{Name: "actions_tlsinfo_gettlscertificate_tls_dial_failure",
		Description: "number of tls dial failure when performing GetTLSCertificate"}
	systemCertPoolFailureCounter = metrics.MetricDefinition{Name: "actions_tlsinfo_gettlscertificate_system_certpool_failure",
		Description: "number of system certpool retrieval failure when performing GetTLSCertificate"}
)

// server implements TLSInfoServer
var _ pb.TLSInfoServer = &server{}

type server struct{}

func (s *server) GetTLSCertificate(ctx context.Context, req *pb.TLSCertificateRequest) (*pb.TLSCertificateChain, error) {
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger := logr.FromContextOrDiscard(ctx)
	caPool, err := x509.SystemCertPool()
	if err != nil {
		recorder.CounterOrLog(ctx, systemCertPoolFailureCounter, 1)
		return nil, fmt.Errorf("failed to get CA Pool: %v", err)
	}
	tlsConf := &tls.Config{
		RootCAs:            caPool,
		InsecureSkipVerify: req.InsecureSkipVerify,
		ServerName:         req.ServerName,
	}
	logger.V(3).Info(fmt.Sprintf("GetTLSCertificate: dialing %s with tls config %+v", req.ServerAddress, tlsConf))

	conn, err := tls.Dial("tcp", req.ServerAddress, tlsConf)
	if err != nil {
		recorder.CounterOrLog(ctx, tlsDialFailureCounter, 1)
		return nil, fmt.Errorf("failed to dial TLS server %s %v", req.ServerAddress, err)
	}

	result := &pb.TLSCertificateChain{}
	state := conn.ConnectionState()
	for _, cert := range state.PeerCertificates {
		protoCert := &pb.TLSCertificate{
			Issuer:    cert.Issuer.String(),
			Subject:   cert.Subject.String(),
			NotBefore: cert.NotBefore.Unix(),
			NotAfter:  cert.NotAfter.Unix(),
			DnsNames:  cert.DNSNames,
		}
		for _, ipAddr := range cert.IPAddresses {
			protoCert.IpAddresses = append(protoCert.IpAddresses, ipAddr.String())
		}

		result.Certificates = append(result.Certificates, protoCert)
	}

	return result, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterTLSInfoServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
