package output

import (
	"context"
	"errors"
	app "github.com/Snowflake-Labs/sansshell/services/network/server/application"
	"net"
	"strconv"
	"strings"
	"time"
)

// TCPClient is implementation of [github.com/Snowflake-Labs/sansshell/services/network/server/application.TCPClientPort] interface
type TCPClient struct {
}

// CheckConnectivity is used to check tcp connectivity from remote machine to specified server
func (p *TCPClient) CheckConnectivity(ctx context.Context, hostname string, port uint8, timeoutSeconds uint32) (*app.TCPConnectivityCheckResult, error) {
	hostToCheck := net.JoinHostPort(hostname, strconv.Itoa(int(port)))

	timeout := time.Duration(timeoutSeconds) * time.Second

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", hostToCheck)

	if err != nil {
		var failReason string
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			failReason = "Connection timed out"
		} else if opErr, ok := err.(*net.OpError); ok && opErr.Op == "dial" {
			if strings.HasSuffix(opErr.Err.Error(), "no such host") {
				failReason = "No such host"
			} else if strings.HasSuffix(opErr.Err.Error(), "connection refused") {
				failReason = "Connection refused"
			}
		}

		if failReason == "" {
			return nil, errors.New("Unexpected error: " + err.Error())
		}

		return &app.TCPConnectivityCheckResult{
			IsOk:       false,
			FailReason: &failReason,
		}, nil
	}

	defer conn.Close()
	return &app.TCPConnectivityCheckResult{
		IsOk: true,
	}, nil
}
