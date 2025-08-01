/* Copyright (c) 2025 Snowflake Inc. All rights reserved.

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
	"testing"
	"time"
)

func TestRoundRemainingTime(t *testing.T) {
	tests := []struct {
		name     string
		future   time.Time
		wantDays int
		wantHrs  int
		wantMins int
	}{
		{
			name:     "1 day",
			future:   time.Now().Add(24 * time.Hour),
			wantDays: 1,
			wantHrs:  0,
			wantMins: 0,
		},
		{
			name:     "2 day 3 hours 30 minutes",
			future:   time.Now().Add(51*time.Hour + 30*time.Minute),
			wantDays: 2,
			wantHrs:  3,
			wantMins: 30,
		},
		{
			name:     "45 minutes",
			future:   time.Now().Add(45 * time.Minute),
			wantDays: 0,
			wantHrs:  0,
			wantMins: 45,
		},
		{
			name:     "1 hour 45 minutes ago",
			future:   time.Now().Add(-(1*time.Hour + 45*time.Minute)),
			wantDays: 0,
			wantHrs:  -1,
			wantMins: -45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, hours, minutes := roundRemainingTime(tt.future)
			if days != tt.wantDays || hours != tt.wantHrs || minutes != tt.wantMins {
				t.Errorf("roundRemainingTime() = (%d, %d, %d), want (%d, %d, %d)",
					days, hours, minutes, tt.wantDays, tt.wantHrs, tt.wantMins)
			}
		})
	}
}
