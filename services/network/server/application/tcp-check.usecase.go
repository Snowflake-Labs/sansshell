package application

import (
	"context"
)

type TCPConnectivityCheckResult struct {
	IsOk       bool
	FailReason *string
}

type TCPClientPort interface {
	CheckConnectivity(ctx context.Context, hostname string, port uint8, timeoutSeconds uint32) (*TCPConnectivityCheckResult, error)
}

type tcpCheckUsecase struct {
	tcpClientPort TCPClientPort
}

type TCPCheckUsecase interface {
	Run(ctx context.Context, hostname string, port uint8, timeoutSeconds uint32) (*TCPConnectivityCheckResult, error)
}

func (t *tcpCheckUsecase) Run(ctx context.Context, hostname string, port uint8, timeoutSeconds uint32) (*TCPConnectivityCheckResult, error) {
	result, err := t.tcpClientPort.CheckConnectivity(ctx, hostname, port, timeoutSeconds)
	return result, err
}

func NewTCPCheckUsecase(client TCPClientPort) TCPCheckUsecase {
	return &tcpCheckUsecase{
		tcpClientPort: client,
	}
}
