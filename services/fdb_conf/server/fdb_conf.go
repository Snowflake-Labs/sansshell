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
	"io/ioutil"
	"os"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/fdb_conf"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/ini.v1"
)

// TODO add section name validator https://apple.github.io/foundationdb/configuration.html#foundationdb-conf

type server struct {
}

func (s *server) Read(_ context.Context, req *pb.ReadRequest) (*pb.FdbConfResponse, error) {
	cfg, err := ini.Load(req.Location.File)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not load config file %s: %v", req.Location.File, err)
	}

	value := cfg.Section(req.Location.Section).Key(req.Location.Key).String()

	return &pb.FdbConfResponse{Value: value}, nil
}

func (s *server) Write(_ context.Context, req *pb.WriteRequest) (*emptypb.Empty, error) {
	section := req.Location.Section
	if section == "" {
		return nil, status.Error(codes.InvalidArgument, "section name can not be empty")
	}

	sectionKey := req.Location.Key
	if sectionKey == "" {
		return nil, status.Error(codes.InvalidArgument, "key name can not be empty")
	}

	sectionKeyVal := req.Value
	// For now, disallow empty values since there are not usecases for that.
	// But maybe, in the future, key = "" will be valid config, just not now
	if sectionKeyVal == "" {
		return nil, status.Error(codes.InvalidArgument, "key value can not be empty")
	}

	cfg, err := ini.Load(req.Location.File)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not load config file %s: %v", req.Location.File, err)
	}

	cfg.Section(section).Key(sectionKey).SetValue(sectionKeyVal)

	if err = atomicSaveTo(cfg, req.Location.File); err != nil {
		return nil, status.Errorf(codes.Internal, "could not save config file %s: %v", req.Location.File, err)
	}

	return &emptypb.Empty{}, nil
}

func (s *server) Delete(_ context.Context, req *pb.DeleteRequest) (*emptypb.Empty, error) {
	section := req.Location.Section
	if section == "" {
		return nil, status.Error(codes.InvalidArgument, "section name can not be empty")
	}

	cfg, err := ini.Load(req.Location.File)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not load config file %s: %v", req.Location.File, err)
	}

	sectionKey := req.Location.Key
	if sectionKey != "" {
		cfg.Section(section).DeleteKey(sectionKey)
	}
	// TODO maybe use a sentinel for section deletion to avoid accidents?
	if sectionKey == "" {
		cfg.DeleteSection(section)
	}

	if err = atomicSaveTo(cfg, req.Location.File); err != nil {
		return nil, status.Errorf(codes.Internal, "could not save config file %s: %v", req.Location.File, err)
	}

	return &emptypb.Empty{}, nil
}

func (s *server) Register(gs *grpc.Server) {
	pb.RegisterFdbConfServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}

func atomicSaveTo(f *ini.File, filename string) error {
	tf, err := ioutil.TempFile("", "fdb_conf")
	if err != nil {
		return err
	}
	defer tf.Close()

	tfilename := tf.Name()
	defer os.Remove(tfilename)

	if err = f.SaveTo(tfilename); err != nil {
		return err
	}

	return os.Rename(tfilename, filename)
}
