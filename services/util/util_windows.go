package util

import (
	"errors"
	"syscall"
)

func getSysProcAttr(options *cmdOptions) (*syscall.SysProcAttr, error) {
	return nil, errors.New("setting uid/gid is unsupported on Windows")
}
