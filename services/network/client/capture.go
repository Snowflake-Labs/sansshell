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

package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	pb "github.com/Snowflake-Labs/sansshell/services/network"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/google/subcommands"
)

type rawStreamCmd struct {
	maxPackets int
	snapLength int
	iface      string
	format     string
	linkType   int
}

func (*rawStreamCmd) Name() string     { return "capture" }
func (*rawStreamCmd) Synopsis() string { return "Capture packets" }
func (*rawStreamCmd) Usage() string {
	return `capture [-c COUNT] [-i IFACE] [-f FORMAT] [FILTER]:
	 capture up to COUNT network packets on interface IFACE, matching the FILTER
	 FILTER is in the format specified by manpage pcap-filter(7)
`
}

func (p *rawStreamCmd) SetFlags(f *flag.FlagSet) {
	f.IntVar(&p.maxPackets, "c", 0, "exit after receiving N packets")
	f.StringVar(&p.iface, "i", "", "network interface to listen on")
	f.StringVar(&p.format, "f", string(textFormat), fmt.Sprintf("output format: %s, %s, %s, %s", textFormat, dumpFormat, pcapFormat, pcapNgFormat))
	f.IntVar(&p.linkType, "link-type", int(layers.LinkTypeEthernet), "link type as per pcap-linktype(7)")

	// 256kB ought to be enough for anybody as defined by libpcap (MAXIMUM_SNAPLEN)
	// https://github.com/the-tcpdump-group/libpcap/blob/89c94d4c7f503387da9840d4c9074262a67fc2ad/pcap-int.h#L125-L151
	f.IntVar(&p.snapLength, "s", 262144, "truncate packets longer than N bytes")
}

const (
	// https://ietf-opsawg-wg.github.io/draft-ietf-opsawg-pcap/draft-ietf-opsawg-pcap.html
	pcapFormat = "pcap"
	// https://ietf-opsawg-wg.github.io/draft-ietf-opsawg-pcap/draft-ietf-opsawg-pcapng.html
	pcapNgFormat = "pcapng"
	// [google/gopacket/Packet.String]
	textFormat = "text"
	// [google/gopacket/Packet.Dump]
	dumpFormat = "dump"
)

type packetWriter interface {
	WritePacket(ci gopacket.CaptureInfo, data []byte) error
}

type textWriter struct {
	Out       io.Writer
	Stringify func(gopacket.Packet) string
	Decoder   gopacket.Decoder
}

// Ensure that textWriter implements packetWriter
var _ packetWriter = (*textWriter)(nil)

func (w textWriter) WritePacket(ci gopacket.CaptureInfo, data []byte) error {
	pkt := gopacket.NewPacket(data, w.Decoder, gopacket.Default)
	fmt.Fprintf(w.Out, "[%s]: %s\n", ci.Timestamp.Local(), w.Stringify(pkt))
	return nil
}

func (p *rawStreamCmd) Execute(ctx context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	state := args[0].(*util.ExecuteState)
	c := pb.NewPacketCaptureClientProxy(state.Conn)

	filter := strings.TrimSpace(strings.Join(f.Args(), " "))
	req := &pb.RawStreamRequest{
		Filter:        filter,
		Interface:     p.iface,
		CountLimit:    int32(p.maxPackets),
		CaptureLength: int32(p.snapLength),
	}

	stream, err := c.RawStreamOneMany(ctx, req)
	if err != nil {
		// Emit this to every error file as it's not specific to a given target.
		for _, e := range state.Err {
			fmt.Fprintln(e, "proxy error:", err)
		}
		return subcommands.ExitFailure
	}

	// initialize writers
	writers := make([]packetWriter, len(state.Out))
	linkType := layers.LinkType(p.linkType)
	for i := range state.Out {
		switch p.format {
		case pcapFormat:
			w := pcapgo.NewWriter(state.Out[i])
			w.WriteFileHeader(uint32(p.snapLength), linkType)
			writers[i] = w
		case pcapNgFormat:
			tgt := state.Conn.Targets[i]
			options := pcapgo.NgWriterOptions{SectionInfo: pcapgo.NgSectionInfo{
				Application: "SansShell",
				Comment:     tgt,
			}}
			iface := pcapgo.NgInterface{
				Name:        p.iface,
				LinkType:    linkType,
				SnapLength:  uint32(p.snapLength),
				Filter:      filter,
				Description: tgt,
			}
			w, err := pcapgo.NewNgWriterInterface(state.Out[i], iface, options)
			if err != nil {
				fmt.Fprintln(state.Err[i], "unable to create a PcapNG writer:", err)
				continue
			}
			defer w.Flush()
			writers[i] = w
		case dumpFormat:
			writers[i] = textWriter{Out: state.Out[i], Decoder: linkType, Stringify: gopacket.Packet.Dump}
		case textFormat:
			writers[i] = textWriter{Out: state.Out[i], Decoder: linkType, Stringify: gopacket.Packet.String}
		default:
			fmt.Fprintln(os.Stderr, "unknown output format:", p.format)
			return subcommands.ExitFailure
		}
	}

	targetsDone := make(map[int]bool)
	exit := subcommands.ExitSuccess
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Emit this to every error file as it's not specific to a given target.
			// But...we only do this for targets that aren't complete. A complete target
			// didn't have an error. i.e. we got N done then the context expired.
			for i, e := range state.Err {
				if !targetsDone[i] {
					fmt.Fprintln(e, "stream error:", err)
				}
			}
			exit = subcommands.ExitFailure
			break
		}
		for _, r := range resp {
			if r.Error != nil && r.Error != io.EOF {
				fmt.Fprintf(state.Err[r.Index], "Target %s (%d) returned error - %v\n", r.Target, r.Index, r.Error)
				targetsDone[r.Index] = true
				// If any target had errors it needs to be reported for that target but we still
				// need to process responses off the channel. Final return code though should
				// indicate something failed.
				exit = subcommands.ExitFailure
				continue
			}
			// At EOF this target is done.
			if r.Error == io.EOF {
				targetsDone[r.Index] = true
				continue
			}

			resp := r.Resp
			metadata := gopacket.CaptureInfo{
				Length:        int(resp.FullLength),
				CaptureLength: len(resp.Data),
				Timestamp:     resp.Timestamp.AsTime(),
			}
			if err := writers[r.Index].WritePacket(metadata, resp.Data); err != nil {
				fmt.Fprintln(state.Err[r.Index], "unable to write packet:", err)
			}
		}
	}
	return exit
}
