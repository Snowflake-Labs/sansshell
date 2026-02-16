/* Copyright (c) 2019 Snowflake Inc. All rights reserved.

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

package server

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

// FallbackDialer is a TargetDialer that tries a primary dialer first,
// and if it fails with a TLS-related error, retries with a fallback dialer.
type FallbackDialer struct {
	primary  TargetDialer
	fallback TargetDialer
	logger   logr.Logger
}

// NewFallbackDialer creates a new FallbackDialer that tries primary first,
// then falls back to fallback on TLS errors.
func NewFallbackDialer(primary, fallback TargetDialer, logger logr.Logger) *FallbackDialer {
	return &FallbackDialer{
		primary:  primary,
		fallback: fallback,
		logger:   logger,
	}
}

// DialContext implements TargetDialer.DialContext with fallback logic.
// It first attempts to dial using the primary dialer. If that fails with
// a TLS-related error (or a deadline exceeded, which can mask a TLS error
// when grpc.WithBlock is used), it retries using the fallback dialer with
// a fresh context so the fallback has its own time budget.
func (f *FallbackDialer) DialContext(ctx context.Context, target string, dialOpts ...grpc.DialOption) (ClientConnCloser, error) {
	conn, err := f.primary.DialContext(ctx, target, dialOpts...)
	if err == nil {
		return conn, nil
	}

	if !IsTLSError(err) {
		return nil, err
	}

	f.logger.Info("primary dial failed with TLS error, attempting fallback",
		"target", target,
		"error", err.Error())

	// Dial without the caller's dialOpts (which may include grpc.WithBlock).
	// A lazy (non-blocking) dial returns immediately; if the fallback creds
	// are wrong, the TLS error will surface at NewStream time in target.go.
	fallbackConn, fallbackErr := f.fallback.DialContext(context.WithoutCancel(ctx), target)
	if fallbackErr != nil {
		f.logger.Error(fallbackErr, "fallback dial also failed", "target", target)
		return nil, err
	}

	f.logger.Info("fallback dial succeeded", "target", target)
	return fallbackConn, nil
}

// IsTLSError checks if the error is related to TLS/certificate issues.
// This function is exported so it can be used by other packages if needed.
func IsTLSError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Common TLS error patterns from Go's crypto/tls and x509 packages,
	// as well as gRPC transport errors
	tlsPatterns := []string{
		// x509 certificate errors
		"certificate",
		"x509:",
		"unknown authority",
		"certificate signed by unknown authority",
		"certificate verify failed",
		"certificate has expired",
		"certificate is not valid",
		"certificate is valid for",
		// TLS handshake errors
		"tls:",
		"TLS",
		"handshake",
		"bad certificate",
		"remote error: tls:",
		// gRPC transport authentication errors
		"authentication handshake failed",
		"credentials require transport level security",
		"transport: authentication handshake failed",
		// When grpc.WithBlock is used, gRPC retries the connection internally
		// until the context deadline, masking the underlying TLS error as a
		// deadline exceeded error. We treat this as a potential TLS error to
		// allow the fallback to attempt with different credentials.
		"context deadline exceeded",
	}

	for _, pattern := range tlsPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
