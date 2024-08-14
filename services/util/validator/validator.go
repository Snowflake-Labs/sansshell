package validator

import (
	"errors"
	"strconv"
	"strings"
)

func ParsePortFromInt(val int) (uint32, error) {
	if val < 1 || val > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}

	return uint32(val), nil
}

func ParsePortFromUint32(val uint32) (uint32, error) {
	if val < 1 || val > 65535 {
		return 0, errors.New("port must be between 1 and 65535")
	}

	return val, nil
}

func ParseHostAndPort(hostAndPort string) (string, uint32, error) {
	artifacts := strings.Split(hostAndPort, ":")
	if len(artifacts) != 2 {
		return "", 0, errors.New("invalid host:port format")
	}

	rawHost := strings.TrimSpace(artifacts[0])
	rawPort := artifacts[1]

	if rawHost == "" {
		return "", 0, errors.New("host cannot be empty")
	}
	host := rawHost

	intPort, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, errors.New("port must be a number")
	}

	port, err := ParsePortFromInt(intPort)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}
