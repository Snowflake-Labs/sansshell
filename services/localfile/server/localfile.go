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

// Package server implements the sansshell 'LocalFile' service.
package server

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"gocloud.dev/blob"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"

	"golang.org/x/sys/unix"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	// AbsolutePathError is a typed error for path errors on file/directory names.
	AbsolutePathError = status.Error(codes.InvalidArgument, "filename path must be absolute and clean")

	// For testing since otherwise tests have to run as root for these.
	chown             = unix.Chown
	changeImmutableOS = changeImmutable

	// ReadTimeout is how long tail should wait on a given poll call
	// before checking context.Err() and possibly looping.
	ReadTimeout = 10 * time.Second
)

// Metrics
const (
	localfileReadFailureCounterName              = "actions_localfile_read_failure"
	localfileReadFailureCounterDesc              = "number of failures when performing localfile.Read"
	localfileStatFailureCounterName              = "actions_localfile_stat_failure"
	localfileStatFailureCounterDesc              = "number of failures when performing localfile.Stat"
	localfileSumFailureCounterName               = "actions_localfile_sum_failure"
	localfileSumFailureCounterDesc               = "number of failures when performing localfile.Sum"
	localfileWriteFailureCounterName             = "actions_localfile_write_failure"
	localfileWriteFailureCounterDesc             = "number of failures when performing localfile.Write"
	localfileCopyFailureCounterName              = "actions_localfile_copy_failure"
	localfileCopyFailureCounterDesc              = "number of failures when performing localfile.Copy"
	localfileListFailureCounterName              = "actions_localfile_copy_failure"
	localfileListFailureCounterDesc              = "number of failures when performing localfile.List"
	localfileRmFailureCounterName                = "actions_localfile_rm_failure"
	localfileRmFailureCounterDesc                = "number of failures when performing localfile.Rm"
	localfileRmDirFailureCounterName             = "actions_localfile_rmdir_failure"
	localfileRmDirFailureCounterDesc             = "number of failures when performing localfile.Rmdir"
	localfileRenameFailureCounterName            = "actions_localfile_rename_failure"
	localfileRenameFailureCounterDesc            = "number of failures when performing localfile.Rename"
	localfileReadlinkFailureCounterName          = "actions_localfile_readlink_failure"
	localfileReadlinkFailureCounterDesc          = "number of failures when performing localfile.Readlink"
	localfileSymlinkFailureCounterName           = "actions_localfile_symlink_failure"
	localfileSymlinkFailureCounterDesc           = "number of failures when performing localfile.Symlink"
	localfileSetFileAttributesFailureCounterName = "actions_localfile_setfileattribute_failure"
	localfileSetFileAttributesFailureCounterDesc = "number of failures when performing localfile.SetFileAttribute"
)

// This encompasses the permission plus the setuid/gid/sticky bits one
// can set on a file/directory.
const modeMask = uint32(fs.ModePerm | fs.ModeSticky | fs.ModeSetuid | fs.ModeSetgid)

// server is used to implement the gRPC server
type server struct{}

// Read returns the contents of the named file
func (s *server) Read(req *pb.ReadActionRequest, stream pb.LocalFile_ReadServer) error {
	ctx := stream.Context()
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	r := req.GetFile()
	t := req.GetTail()

	var file string
	var offset, length int64
	switch {
	case r != nil:
		file = r.Filename
		offset = r.Offset
		length = r.Length
	case t != nil:
		file = t.Filename
		offset = t.Offset
	default:
		errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "invalid_args"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
		}
		return status.Error(codes.InvalidArgument, "must supply a ReadRequest or a TailRequest")
	}

	logger.Info("read request", "filename", file)
	if err := util.ValidPath(file); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "invalid_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
		}
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "open_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
		}
		return status.Errorf(codes.Internal, "can't open file %s: %v", file, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "close_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
			}
			logger.Error(err, "file.Close()", "file", file)
		}
	}()

	// Seek forward if requested
	if offset != 0 {
		whence := 0
		// If negative we're tailing from the end so
		// negate the sign and set whence.
		if offset < 0 {
			whence = 2
		}
		if _, err := f.Seek(offset, whence); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "seek_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
			}
			return status.Errorf(codes.Internal, "can't seek for file %s: %v", file, err)
		}
	}

	max := length
	if max == 0 {
		max = math.MaxInt64
	}

	buf := make([]byte, util.StreamingChunkSize)

	reader := io.LimitReader(f, max)

	td, closer, err := dataPrep(f)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "dataprep_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
		}
		return err
	}
	defer closer()

	for {
		n, err := reader.Read(buf)
		// If we got EOF we're done for normal reads and wait for tails.
		if err == io.EOF {
			// If we're not tailing then we're done.
			if r != nil {
				break
			}
			if err := dataReady(td, stream); err != nil {
				errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
				if errRegister != nil {
					logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
				}
				errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "dataready_err"))
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
				}
				return err
			}
			continue
		}

		if err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "read_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
			}
			return status.Errorf(codes.Internal, "can't read file %s: %v", file, err)
		}

		// Only send over the number of bytes we actually read or
		// else we'll send over garbage in the last packet potentially.
		if err := stream.Send(&pb.ReadReply{Contents: buf[:n]}); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileReadFailureCounterName, localfileReadFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileReadFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileReadFailureCounterName, 1, attribute.String("reason", "stream_send_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileReadFailureCounterName)
			}
			return status.Errorf(codes.Internal, "can't send on stream for file %s: %v", file, err)
		}

		// If we got back less than a full chunk we're done for non-tail cases.
		if n < util.StreamingChunkSize {
			if r != nil {
				break
			}
		}
	}
	return nil
}

func (s *server) Stat(stream pb.LocalFile_StatServer) error {
	ctx := stream.Context()
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileStatFailureCounterName, localfileStatFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileStatFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileStatFailureCounterName, 1, attribute.String("reason", "stream_recv_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileStatFailureCounterName)
			}
			return status.Errorf(codes.Internal, "stat: recv error %v", err)
		}

		logger.Info("stat", "filename", req.Filename)
		if err := util.ValidPath(req.Filename); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileStatFailureCounterName, localfileStatFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileStatFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileStatFailureCounterName, 1, attribute.String("reason", "invalid_path"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileStatFailureCounterName)
			}
			return AbsolutePathError
		}
		resp, err := osStat(req.Filename, !req.FollowLinks)
		if err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileStatFailureCounterName, localfileStatFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileStatFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileStatFailureCounterName, 1, attribute.String("reason", "stat_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileStatFailureCounterName)
			}
			return err
		}
		if err := stream.Send(resp); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileStatFailureCounterName, localfileStatFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileStatFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileStatFailureCounterName, 1, attribute.String("reason", "stream_send_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileStatFailureCounterName)
			}
			return status.Errorf(codes.Internal, "stat: send error %v", err)
		}
	}
}

func (s *server) Sum(stream pb.LocalFile_SumServer) error {
	ctx := stream.Context()
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "recv_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
			}
			return status.Errorf(codes.Internal, "sum: recv error %v", err)
		}
		logger.Info("sum request", "file", req.Filename, "sumtype", req.SumType.String())
		if err := util.ValidPath(req.Filename); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "invalid_path"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
			}
			return AbsolutePathError
		}
		out := &pb.SumReply{
			SumType:  req.SumType,
			Filename: req.Filename,
		}
		var hasher hash.Hash
		switch req.SumType {
		// default to sha256 for unspecified
		case pb.SumType_SUM_TYPE_UNKNOWN, pb.SumType_SUM_TYPE_SHA256:
			hasher = sha256.New()
			out.SumType = pb.SumType_SUM_TYPE_SHA256
		case pb.SumType_SUM_TYPE_MD5:
			hasher = md5.New()
		case pb.SumType_SUM_TYPE_SHA512_256:
			hasher = sha512.New512_256()
		case pb.SumType_SUM_TYPE_CRC32IEEE:
			hasher = crc32.NewIEEE()
		default:
			errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "invalid_sumtype"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
			}
			return status.Errorf(codes.InvalidArgument, "invalid sum type value %d", req.SumType)
		}
		if err := func() error {
			f, err := os.Open(req.Filename)
			if err != nil {
				errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
				if errRegister != nil {
					logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
				}
				errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "open_err"))
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
				}
				return err
			}
			defer f.Close()
			if _, err := io.Copy(hasher, f); err != nil {
				logger.Error(err, "io.Copy", "file", req.Filename)
				errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
				if errRegister != nil {
					logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
				}
				errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "copy_err"))
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
				}
				return status.Errorf(codes.Internal, "copy/read error: %v", err)
			}
			out.Sum = hex.EncodeToString(hasher.Sum(nil))
			return nil
		}(); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "sum_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
			}
			return status.Errorf(codes.Internal, "can't create sum: %v", err)
		}
		if err := stream.Send(out); err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileSumFailureCounterName, localfileSumFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileSumFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileSumFailureCounterName, 1, attribute.String("reason", "stream_send_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileSumFailureCounterName)
			}
			return status.Errorf(codes.Internal, "sum: send error %v", err)
		}
	}
}

func setupOutput(a *pb.FileAttributes) (*os.File, *immutableState, error) {
	// Validate path. We'll go ahead and write the data to a tmpfile and
	// do the overwrite check when we rename below.
	filename := a.Filename
	if err := util.ValidPath(filename); err != nil {
		return nil, nil, err
	}

	f, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "can't create tmp file: %v", err)
	}

	// Set owner/gid/perms now since we have an open FD to the file and we don't want
	// to accidentally leave this in another otherwise default state.
	// Except we don't trigger immutable now or we won't be able to write to it.
	immutable, err := validateAndSetAttrs(f.Name(), a.Attributes, false)
	return f, immutable, err
}

func finalizeFile(d *pb.FileWrite, f *os.File, filename string, immutable *immutableState) error {
	if err := f.Close(); err != nil {
		return status.Errorf(codes.Internal, "error closing %s - %v", f.Name(), err)
	}

	// Do one final check (though racy) to see if the file exists.
	_, err := os.Stat(filename)
	if err == nil && !d.Overwrite {
		return status.Errorf(codes.Internal, "file %s exists and overwrite set to false", filename)
	}

	// Rename tmp file to real destination.
	if err := os.Rename(f.Name(), filename); err != nil {
		return status.Errorf(codes.Internal, "error renaming %s -> %s - %v", f.Name(), filename, err)
	}

	// Now set immutable if requested.
	if immutable.setImmutable && immutable.immutable {
		if err := changeImmutableOS(filename, immutable.immutable); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) Write(stream pb.LocalFile_WriteServer) (retErr error) {
	ctx := stream.Context()
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	var f *os.File
	var d *pb.FileWrite
	cleanup := func() {
		if retErr != nil {
			if f != nil {
				// Close and then remove the tmpfile since some error happened.
				f.Close()
				os.Remove(f.Name())
			}
		}
	}
	defer cleanup()

	var immutable *immutableState
	var filename string

	// We must get a description. If this isn't one we bail now.
	req, err := stream.Recv()
	// Don't check for EOF here as getting one now is a real error (we haven't gotten any packets)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "stream_recv_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
		}
		return status.Errorf(codes.Internal, "write: recv error %v", err)
	}

	d = req.GetDescription()
	if d == nil {
		errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "get_desc_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
		}
		return status.Errorf(codes.InvalidArgument, "must send a description block first")

	}
	a := d.GetAttrs()
	if a == nil {
		errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "get_attrs_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
		}
		return status.Errorf(codes.InvalidArgument, "must send attrs in description")
	}
	filename = a.Filename
	logger.Info("write file", filename)
	f, immutable, err = setupOutput(a)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "setup_output_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
		}
		return err
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "stream_recv_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
			}
			return status.Errorf(codes.Internal, "write: recv error %v", err)
		}

		switch {
		case req.GetDescription() != nil:
			errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "mult_desc_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
			}
			return status.Errorf(codes.InvalidArgument, "can't send multiple description blocks")
		case req.GetContents() != nil:
			n, err := f.Write(req.GetContents())
			if err != nil {
				errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
				if errRegister != nil {
					logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
				}
				errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "write_err"))
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
				}
				return status.Errorf(codes.Internal, "write error: %v", err)
			}
			if got, want := n, len(req.GetContents()); got != want {
				errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
				if errRegister != nil {
					logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
				}
				errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "short_write"))
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
				}
				return status.Errorf(codes.Internal, "short write. expected %d but only wrote %d", got, want)
			}
		default:
			errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "missing_desc_content"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
			}
			return status.Error(codes.InvalidArgument, "Must supply either a Description or Contents")
		}
	}

	// Finalize to the final destination and possibly set immutable.
	if err := finalizeFile(d, f, filename, immutable); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileWriteFailureCounterName, localfileWriteFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileWriteFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileWriteFailureCounterName, 1, attribute.String("reason", "finalize_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileWriteFailureCounterName)
		}
		return err
	}
	return nil
}

func (s *server) Copy(ctx context.Context, req *pb.CopyRequest) (_ *emptypb.Empty, retErr error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	if req.GetDestination() == nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "missing_dst"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Error(codes.InvalidArgument, "destination must be filled in")
	}
	d := req.GetDestination()

	if req.Bucket == "" {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "missing_bucket"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Error(codes.InvalidArgument, "bucket must be filled in")
	}
	if req.Key == "" {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "missing_key"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Error(codes.InvalidArgument, "key must be filled in")
	}

	a := d.GetAttrs()
	if a == nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "missing_attrs"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Errorf(codes.InvalidArgument, "must send attrs")
	}
	filename := a.Filename
	logger.Info("copy file", filename)
	f, immutable, err := setupOutput(a)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "setup_output_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, err
	}
	cleanup := func() {
		if retErr != nil {
			if f != nil {
				// Close and then remove the tmpfile since some error happened.
				f.Close()
				os.Remove(f.Name())
			}
		}
	}
	defer cleanup()

	// Copy file over
	b, err := blob.OpenBucket(ctx, req.Bucket)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "open_bucket_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "can't open bucket %s - %v", req.Bucket, err)
	}

	// Something else may error so append onto it.
	defer func() {
		if err := b.Close(); err != nil {
			retErr = fmt.Errorf("%w %v", retErr, err)
		}
	}()

	reader, err := b.NewReader(ctx, req.Key, nil)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "open_bucket_key_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "can't open key %s in bucket %s - %v", req.Key, req.Bucket, err)
	}

	// Something else may error so append onto it.
	defer func() {
		if err := reader.Close(); err != nil {
			retErr = fmt.Errorf("%w %v", retErr, err)
		}
	}()

	written, err := io.Copy(f, reader)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "copy_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "can't copy from bucket %s/%s Only wrote %d bytes - %v", req.Bucket, req.Key, written, err)
	}

	// Finalize to the final destination and possibly set immutable.
	if err := finalizeFile(d, f, filename, immutable); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileCopyFailureCounterName, localfileCopyFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileCopyFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileCopyFailureCounterName, 1, attribute.String("reason", "finalize_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileCopyFailureCounterName)
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *server) List(req *pb.ListRequest, server pb.LocalFile_ListServer) error {
	ctx := server.Context()
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if req.Entry == "" {
		errRegister := recorder.RegisterInt64Counter(localfileListFailureCounterName, localfileListFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileListFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileListFailureCounterName, 1, attribute.String("reason", "missing_entry"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileListFailureCounterName)
		}
		return status.Errorf(codes.InvalidArgument, "filename must be filled in")
	}
	if err := util.ValidPath(req.Entry); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileListFailureCounterName, localfileListFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileListFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileListFailureCounterName, 1, attribute.String("reason", "invalid_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileListFailureCounterName)
		}
		return err
	}

	// We always send back the entry first.
	logger.Info("ls", "filename", req.Entry)
	resp, err := osStat(req.Entry, false)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileListFailureCounterName, localfileListFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileListFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileListFailureCounterName, 1, attribute.String("reason", "stat_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileListFailureCounterName)
		}
		return err
	}
	if err := server.Send(&pb.ListReply{Entry: resp}); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileListFailureCounterName, localfileListFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileListFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileListFailureCounterName, 1, attribute.String("reason", "send_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileListFailureCounterName)
		}
		return status.Errorf(codes.Internal, "list: send error %v", err)
	}

	// If it's directory we'll open it and go over its entries.
	if fs.FileMode(resp.Mode).IsDir() {
		entries, err := os.ReadDir(req.Entry)
		if err != nil {
			errRegister := recorder.RegisterInt64Counter(localfileListFailureCounterName, localfileListFailureCounterDesc)
			if errRegister != nil {
				logger.V(1).Error(errRegister, "failed to register "+localfileListFailureCounterName)
			}
			errCounter := recorder.AddInt64Counter(ctx, localfileListFailureCounterName, 1, attribute.String("reason", "read_dir_err"))
			if errCounter != nil {
				logger.V(1).Error(errCounter, "failed to add counter "+localfileListFailureCounterName)
			}
			return status.Errorf(codes.Internal, "readdir: %v", err)
		}
		// Only do one level so iterate these and we're done.
		for _, e := range entries {
			name := filepath.Join(req.Entry, e.Name())
			logger.Info("ls", "filename", name)
			// Use lstat so that we don't return misleading directory contents from
			// following symlinks.
			resp, err := osStat(name, true)
			if err != nil {
				return err
			}
			if err := server.Send(&pb.ListReply{Entry: resp}); err != nil {
				errCounter := recorder.AddInt64Counter(ctx, localfileListFailureCounterName, 1, attribute.String("reason", "send_err"))
				if errCounter != nil {
					logger.V(1).Error(errCounter, "failed to add counter "+localfileListFailureCounterName)
				}
				return status.Errorf(codes.Internal, "list: send error %v", err)
			}
		}
	}
	return nil
}

// immutableState tracks the parsed state of immutable from the slice of FileAttribute.
type immutableState struct {
	setImmutable bool // Whether immutable was set (so we should change state).
	immutable    bool // The immutable value (only applies if setImmutable is true).
}

func validateAndSetAttrs(filename string, attrs []*pb.FileAttribute, doImmutable bool) (*immutableState, error) {
	uid, gid := int(-1), int(-1)
	setMode, setImmutable, immutable := false, false, false
	mode := uint32(0)

	for _, attr := range attrs {
		switch a := attr.Value.(type) {
		case *pb.FileAttribute_Uid:
			if uid != -1 {
				return nil, status.Error(codes.InvalidArgument, "cannot set uid/username more than once")
			}
			uid = int(a.Uid)
		case *pb.FileAttribute_Username:
			if uid != -1 {
				return nil, status.Error(codes.InvalidArgument, "cannot set uid/username more than once")
			}
			u, err := user.Lookup(a.Username)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "unknown username %s: %v", a.Username, err)
			}
			id, err := strconv.Atoi(u.Uid)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "can't parse uid %s from lookup: %v", u.Uid, err)
			}
			uid = id
		case *pb.FileAttribute_Gid:
			if gid != -1 {
				return nil, status.Error(codes.InvalidArgument, "cannot set gid/group more than once")
			}
			gid = int(a.Gid)
		case *pb.FileAttribute_Group:
			if gid != -1 {
				return nil, status.Error(codes.InvalidArgument, "cannot set gid/group more than once")
			}
			g, err := user.LookupGroup(a.Group)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "unknown group %s: %v", a.Group, err)
			}
			id, err := strconv.Atoi(g.Gid)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "can't parse gid %s from lookup: %v", g.Gid, err)
			}
			gid = id
		case *pb.FileAttribute_Mode:
			if setMode {
				return nil, status.Error(codes.InvalidArgument, "cannot set mode more than once")
			}
			mode = a.Mode
			setMode = true
		case *pb.FileAttribute_Immutable:
			if setImmutable {
				return nil, status.Error(codes.InvalidArgument, "cannot set immutable more than once")
			}
			immutable = a.Immutable
			setImmutable = true
		}
	}

	if uid != -1 || gid != -1 {
		if err := chown(filename, uid, gid); err != nil {
			return nil, status.Errorf(codes.Internal, "error from chown: %v", err)
		}
	}

	if setMode {
		if err := unix.Chmod(filename, mode&modeMask); err != nil {
			return nil, status.Errorf(codes.Internal, "error from chmod: %v", err)
		}
	}

	if doImmutable && setImmutable {
		if err := changeImmutableOS(filename, immutable); err != nil {
			return nil, err
		}
	}
	return &immutableState{
		setImmutable: setImmutable,
		immutable:    immutable,
	}, nil
}

func (s *server) SetFileAttributes(ctx context.Context, req *pb.SetFileAttributesRequest) (*emptypb.Empty, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	if req.Attrs == nil {
		errRegister := recorder.RegisterInt64Counter(localfileSetFileAttributesFailureCounterName, localfileSetFileAttributesFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileSetFileAttributesFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileSetFileAttributesFailureCounterName, 1, attribute.String("reason", "missing_attrs"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileSetFileAttributesFailureCounterName)
		}
		return nil, status.Error(codes.InvalidArgument, "attrs must be filled in")
	}
	p := req.Attrs.Filename
	if p == "" {
		errRegister := recorder.RegisterInt64Counter(localfileSetFileAttributesFailureCounterName, localfileSetFileAttributesFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileSetFileAttributesFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileSetFileAttributesFailureCounterName, 1, attribute.String("reason", "missing_filename"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileSetFileAttributesFailureCounterName)
		}
		return nil, status.Error(codes.InvalidArgument, "filename must be filled in")
	}
	if err := util.ValidPath(p); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileSetFileAttributesFailureCounterName, localfileSetFileAttributesFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileSetFileAttributesFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileSetFileAttributesFailureCounterName, 1, attribute.String("reason", "invalid_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileSetFileAttributesFailureCounterName)
		}
		return nil, err
	}

	// Don't care about immutable state as we set it if it came across.
	if _, err := validateAndSetAttrs(p, req.Attrs.Attributes, true); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileSetFileAttributesFailureCounterName, localfileSetFileAttributesFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileSetFileAttributesFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileSetFileAttributesFailureCounterName, 1, attribute.String("reason", "set_attrs_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileSetFileAttributesFailureCounterName)
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *server) Rm(ctx context.Context, req *pb.RmRequest) (*emptypb.Empty, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger.Info("rm request", "filename", req.Filename)
	if err := util.ValidPath(req.Filename); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRmFailureCounterName, localfileRmFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRmFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRmFailureCounterName, 1, attribute.String("reason", "invalid_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRmFailureCounterName)
		}
		return nil, err
	}
	err := unix.Unlink(req.Filename)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRmFailureCounterName, localfileRmFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRmFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRmFailureCounterName, 1, attribute.String("reason", "unlink_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRmFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "unlink error: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *server) Rmdir(ctx context.Context, req *pb.RmdirRequest) (*emptypb.Empty, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger.Info("rmdir request", "directory", req.Directory)
	if err := util.ValidPath(req.Directory); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRmDirFailureCounterName, localfileRmDirFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRmDirFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRmDirFailureCounterName, 1, attribute.String("reason", "invalid_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRmDirFailureCounterName)
		}
		return nil, err
	}
	err := unix.Rmdir(req.Directory)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRmDirFailureCounterName, localfileRmDirFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRmDirFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRmDirFailureCounterName, 1, attribute.String("reason", "rmdir_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRmDirFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "rmdir error: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *server) Rename(ctx context.Context, req *pb.RenameRequest) (*emptypb.Empty, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger.Info("rename request", "old", req.OriginalName, "new", req.DestinationName)
	if err := util.ValidPath(req.OriginalName); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRenameFailureCounterName, localfileRenameFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRenameFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRenameFailureCounterName, 1, attribute.String("reason", "invalid_original_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRenameFailureCounterName)
		}
		return nil, err
	}
	if err := util.ValidPath(req.DestinationName); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRenameFailureCounterName, localfileRenameFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRenameFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRenameFailureCounterName, 1, attribute.String("reason", "invalid_dst_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRenameFailureCounterName)
		}
		return nil, err
	}
	err := unix.Rename(req.OriginalName, req.DestinationName)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileRenameFailureCounterName, localfileRenameFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileRenameFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileRenameFailureCounterName, 1, attribute.String("reason", "rename_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileRenameFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "rename error: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *server) Readlink(ctx context.Context, req *pb.ReadlinkRequest) (*pb.ReadlinkReply, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger.Info("readlink", "filename", req.Filename)
	if err := util.ValidPath(req.Filename); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileReadlinkFailureCounterName, localfileReadlinkFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadlinkFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadlinkFailureCounterName, 1, attribute.String("reason", "invalid_original_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadlinkFailureCounterName)
		}
		return nil, err
	}
	stat, err := os.Lstat(req.Filename)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileReadlinkFailureCounterName, localfileReadlinkFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadlinkFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadlinkFailureCounterName, 1, attribute.String("reason", "lstat_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadlinkFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "stat error: %v", err)
	}
	if stat.Mode()&os.ModeSymlink != os.ModeSymlink {
		errRegister := recorder.RegisterInt64Counter(localfileReadlinkFailureCounterName, localfileReadlinkFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadlinkFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadlinkFailureCounterName, 1, attribute.String("reason", "not_symlink"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadlinkFailureCounterName)
		}
		return nil, status.Errorf(codes.FailedPrecondition, "%v is not a symlink", req.Filename)
	}
	linkvalue, err := os.Readlink(req.Filename)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileReadlinkFailureCounterName, localfileReadlinkFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileReadlinkFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileReadlinkFailureCounterName, 1, attribute.String("reason", "readlink_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileReadlinkFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "readlink error: %v", err)
	}
	return &pb.ReadlinkReply{Linkvalue: linkvalue}, nil
}

func (s *server) Symlink(ctx context.Context, req *pb.SymlinkRequest) (*emptypb.Empty, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)
	logger.Info("symlink", "target", req.Target, "linkname", req.Linkname)
	// We only check linkname because creating a symbolic link with a relative
	// target is a valid use case.
	if err := util.ValidPath(req.Linkname); err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileSymlinkFailureCounterName, localfileSymlinkFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileSymlinkFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileSymlinkFailureCounterName, 1, attribute.String("reason", "invalid_path"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileSymlinkFailureCounterName)
		}
		return nil, err
	}
	err := os.Symlink(req.Target, req.Linkname)
	if err != nil {
		errRegister := recorder.RegisterInt64Counter(localfileSymlinkFailureCounterName, localfileSymlinkFailureCounterDesc)
		if errRegister != nil {
			logger.V(1).Error(errRegister, "failed to register "+localfileSymlinkFailureCounterName)
		}
		errCounter := recorder.AddInt64Counter(ctx, localfileSymlinkFailureCounterName, 1, attribute.String("reason", "symlink_err"))
		if errCounter != nil {
			logger.V(1).Error(errCounter, "failed to add counter "+localfileSymlinkFailureCounterName)
		}
		return nil, status.Errorf(codes.Internal, "symlink error: %v", err)
	}
	return &emptypb.Empty{}, nil
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterLocalFileServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
