//go:build !(linux || darwin)
// +build !linux,!darwin

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
	"os"
	"syscall"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
)

// osStat is the platform agnostic version which uses basic os.Stat.
// As a result immutable bits cannot be returned.
func osStat(path string) (*pb.StatReply, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stat: os.Stat error %v", err)
	}
	// If a system doesn't support this an OS specific version of osStat needs to be
	// written which simulates stat().
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok || stat_t == nil {
		return nil, status.Error(codes.Unimplemented, "stat not supported")
	}
	resp := &pb.StatReply{
		Filename: path,
		Size:     stat.Size(),
		Mode:     uint32(stat.Mode()),
		Modtime:  timestamppb.New(stat.ModTime()),
		Uid:      stat_t.Uid,
		Gid:      stat_t.Gid,
	}
	return resp, nil
}

// changeImmutable is the default implementation for changing
// immutable bits (which is unsupported).
func changeImmutable(path string, immutable bool) error {
	return status.Error(codes.Unimplemented, "immutable not supported")
}

// dataPrep should be called before entering a loop watching a file.
// It returns an opaque object to pass to dataReady() and a function
// which should be run on exit (i.e. defer it).
func dataPrep(f *os.File) (interface{}, func(), error) {
	return nil, func() {}, nil
}

// dataReady is the OS specific version to indicate the given
// file has more data. In the generic case we simply sleep, check the
// stream and then return no matter what (assuming the file was already
// at EOF).
func dataReady(_ interface{}, stream pb.LocalFile_ReadServer) error {
	// We sleep for READ_TIMEOUT_SEC between calls as there's no good
	// way to poll on a file. Once it reaches EOF it's always readable
	// (you just get EOF). We have to poll like this so we can check
	// the context state and return if it's canclled.
	if stream.Context().Err() != nil {
		return stream.Context().Err()
	}
	time.Sleep(READ_TIMEOUT_SEC * time.Second)
	// Time to try again.
	return nil
}
