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
	pb "github.com/Snowflake-Labs/sansshell/services/localfile"
	"github.com/Snowflake-Labs/sansshell/services/util"
	"github.com/Snowflake-Labs/sansshell/telemetry/metrics"
	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	localfileShredFailureCounter = metrics.MetricDefinition{
		Name:        "actions_localfile_shred_failure",
		Description: "number of failures when performing localfile.Shred",
	}
)

func (s *server) Shred(ctx context.Context, req *pb.ShredRequest) (*emptypb.Empty, error) {
	logger := logr.FromContextOrDiscard(ctx)
	recorder := metrics.RecorderFromContextOrNoop(ctx)

	if err := util.ValidPath(req.Filename); err != nil {
		recorder.CounterOrLog(ctx, localfileShredFailureCounter, 1, attribute.String("reason", "invalid_path"))
		return nil, err
	}

	// TODO: check if hardlink
	// TODO: check if not a /dev or other reserved dirs based file

	logger.Info("shred local file",
		"filename", req.Filename,
		"force", req.Force, "zero", req.Zero,
		"remove", req.Remove)

	return &emptypb.Empty{}, nil
}
