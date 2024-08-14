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

package cli

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDotPreloaderCtrl_Start(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		isInteractive  bool
		expectedOutput string
	}{
		{
			name:           "It should write just provided message in NOT interactive mode",
			message:        "Some preloader",
			isInteractive:  false,
			expectedOutput: "Some preloader...\n",
		},
		{
			name:           "It should write provided message several times with special ANSI codes to handle animation",
			message:        "Some preloader",
			isInteractive:  true,
			expectedOutput: "\r\u001B[K\rSome preloader.\r\u001B[K\rSome preloader..\r\u001B[K\rSome preloader...",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var buf bytes.Buffer
			ctrl := &dotPreloaderCtrl{
				message:            test.message,
				interactiveMode:    test.isInteractive,
				animationFrameRate: 1 * time.Second,
				isActive:           false,
				isActiveMu:         sync.Mutex{},
				outputWriter:       &buf,
			}

			// ACT
			defer ctrl.Stop() // it is required to stop the preloader and kill gorutine in interactive mode
			ctrl.Start()
			time.Sleep(2200 * time.Millisecond)
			result := buf.String()

			// ASSERT
			if result != test.expectedOutput {
				escapedResult := strings.ReplaceAll(strings.ReplaceAll(result, "\r", "\\r"), "\u001B[K", "\\u001B[K")
				escepedExpectedOutput := strings.ReplaceAll(strings.ReplaceAll(test.expectedOutput, "\r", "\\r"), "\u001B[K", "\\u001B[K")
				t.Errorf("Got \"%s\", expected \"%s\"", escapedResult, escepedExpectedOutput)
			}
		})
	}
}

func TestDotPreloaderCtrl_Stop(t *testing.T) {
	t.Run("It should make preloader inactive", func(t *testing.T) {
		// ARRANGE
		var buf bytes.Buffer
		ctrl := &dotPreloaderCtrl{
			message:            "Some",
			interactiveMode:    false,
			animationFrameRate: 1 * time.Second,
			isActive:           false,
			isActiveMu:         sync.Mutex{},
			outputWriter:       &buf,
		}

		// ACT
		ctrl.Start()
		isActiveBeforeStop := ctrl.isActive
		time.Sleep(1000 * time.Millisecond)
		ctrl.Stop()
		isActiveAfterStop := ctrl.isActive

		// ASSERT
		if isActiveBeforeStop != true {
			t.Errorf("Got \"%t\", expected \"%t\"", isActiveBeforeStop, true)
		}

		if isActiveAfterStop != false {
			t.Errorf("Got \"%t\", expected \"%t\"", isActiveBeforeStop, false)
		}
	})
}

func TestDotPreloaderCtrl_StartWith(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		stopMessage    string
		isInteractive  bool
		expectedEndLog string
	}{
		{
			name:           "It should write end message in the end in NOT interactive mode",
			message:        "Some preloader",
			stopMessage:    "Done",
			isInteractive:  false,
			expectedEndLog: "Done",
		},
		{
			name:           "It should write end message and ANSI codes to clean up terminal to handle animation codes to handle animation",
			message:        "Some preloader",
			stopMessage:    "Done",
			isInteractive:  true,
			expectedEndLog: "\r\u001B[K\rDone",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// ARRANGE
			var buf bytes.Buffer
			ctrl := &dotPreloaderCtrl{
				message:            test.message,
				interactiveMode:    test.isInteractive,
				animationFrameRate: 1 * time.Second,
				isActive:           false,
				isActiveMu:         sync.Mutex{},
				outputWriter:       &buf,
			}

			// ACT
			ctrl.Start()
			time.Sleep(2200 * time.Millisecond)
			inprogressOutput := buf.String()
			ctrl.StopWith(test.stopMessage)
			finalOutput := buf.String()

			// ASSERT
			actualEndMessage, _ := strings.CutPrefix(finalOutput, inprogressOutput)
			if actualEndMessage != test.expectedEndLog {
				escapedResult := strings.ReplaceAll(strings.ReplaceAll(actualEndMessage, "\r", "\\r"), "\u001B[K", "\\u001B[K")
				escepedExpectedOutput := strings.ReplaceAll(strings.ReplaceAll(test.expectedEndLog, "\r", "\\r"), "\u001B[K", "\\u001B[K")
				t.Errorf("Got \"%s\", expected \"%s\"", escapedResult, escepedExpectedOutput)
			}
		})
	}
}
