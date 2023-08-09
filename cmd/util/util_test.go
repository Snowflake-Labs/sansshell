package util

import (
	"log"
	"testing"
	"time"

	"github.com/Snowflake-Labs/sansshell/testing/testutil"
)

func TestValidateAndAddPortAndTimeout(t *testing.T) {
	defer func() { logFatalf = log.Fatalf }() // replace the original func after this test
	tests := []struct {
		name           string
		s              string
		port           int
		dialTimeout    time.Duration
		expectFatal    bool
		expectedResult string
	}{
		{
			name:           "port and timeout exists, shouldn't change anything",
			s:              "localhost:9999;12s",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:9999;12s",
		},
		{
			name:           "port doesn't exists, should be added",
			s:              "localhost;12s",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:2345;12s",
		},
		{
			name:           "timeout doesn't exists, should be added",
			s:              "localhost:9999",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:9999;30s",
		},
		{
			name:           "port and timeout don't exist, both should be added",
			s:              "localhost",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "localhost:2345;30s",
		},
		{
			name:           "invalid timeout without port, should fatal",
			s:              "localhost;3030",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "invalid timeout with port, should fatal",
			s:              "localhost:9999;3030",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "empty string, should fatal",
			s:              "",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "semicolon only, should fatal",
			s:              ";",
			port:           2345,
			dialTimeout:    time.Second * 30,
			expectedResult: "",
			expectFatal:    true,
		},
		{
			name:           "zero timeout shouldn't be added",
			s:              "localhost",
			port:           2345,
			dialTimeout:    0,
			expectedResult: "localhost:2345",
			expectFatal:    false,
		},
	}
	for _, tc := range tests {
		fatalCalled := false
		logFatalf = func(format string, v ...any) {
			fatalCalled = true
		}
		result := ValidateAndAddPortAndTimeout(tc.s, tc.port, tc.dialTimeout)
		if tc.expectFatal {
			// if expect fatal, no need to validate result
			testutil.DiffErr(tc.name, fatalCalled, tc.expectFatal, t)
		} else {
			testutil.DiffErr(tc.name, result, tc.expectedResult, t)
		}
	}
}
