/* Copyright (c) 2024 Snowflake Inc. All rights reserved.

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

// Package server implements the SansShell `network` service.
package server

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Snowflake-Labs/sansshell/services"
	pb "github.com/Snowflake-Labs/sansshell/services/network"
)

// Returns name of the first active non-loopback interface or an empty string if no such inteface is found.
func defaultCaptureInterface() string {
	if interfaces, err := fetchInterfaces(); err == nil {
		for _, iface := range interfaces {
			if iface.Flags&(net.FlagUp|net.FlagRunning) != 0 && iface.Flags&net.FlagLoopback == 0 {
				return iface.Name
			}
		}
	}
	return ""
}

// server is used to implement the gRPC server
type server struct{}

var getPcapHandle = getLivePcapHandle

func getLivePcapHandle(iface string, snaplen int32) (*pcap.Handle, error) {
	return pcap.OpenLive(iface, snaplen, true, pcap.BlockForever)
}

func (s *server) RawStream(req *pb.RawStreamRequest, stream pb.PacketCapture_RawStreamServer) error {
	logger := logr.FromContextOrDiscard(stream.Context())
	if req.Interface == "" {
		req.Interface = defaultCaptureInterface()
		logger.V(3).Info("using default capture interface", "interface", req.Interface)
	}
	handle, err := getPcapHandle(req.Interface, req.CaptureLength)
	if err != nil {
		return err
	}
	if req.Filter != "" {
		if err := handle.SetBPFFilter(req.Filter); err != nil {
			return err
		}
	}
	pktSource := gopacket.NewPacketSource(handle, handle.LinkType())
	if err != nil {
		return err
	}
	pkgCount := 0
	for packet := range pktSource.Packets() {
		pkgCount += 1
		ci := packet.Metadata().CaptureInfo
		reply := &pb.RawStreamReply{
			Data:       packet.Data(),
			Timestamp:  timestamppb.New(ci.Timestamp),
			FullLength: int32(ci.Length),
		}
		if err := stream.Send(reply); err != nil {
			return err
		}
		if req.CountLimit > 0 && pkgCount > int(req.CountLimit) {
			break
		}
	}
	return nil
}

// Package-level interfaces call that can be subsituted in tests
var fetchInterfaces = net.Interfaces

func (s *server) ListInterfaces(_ context.Context, _ *emptypb.Empty) (*pb.ListInterfacesReply, error) {
	ifaces, err := fetchInterfaces()
	var r []*pb.Interface
	for _, iface := range ifaces {
		r = append(r, &pb.Interface{
			Index:  int32(iface.Index),
			Name:   iface.Name,
			Mtu:    int32(iface.MTU),
			Hwaddr: iface.HardwareAddr,
			Flags:  uint32(iface.Flags),
		})
	}
	return &pb.ListInterfacesReply{Interfaces: r}, err
}

// Register is called to expose this handler to the gRPC server
func (s *server) Register(gs *grpc.Server) {
	pb.RegisterPacketCaptureServer(gs, s)
}

func init() {
	services.RegisterSansShellService(&server{})
}
