/*
Copyright (c) 2019 Snowflake Inc. All rights reserved.

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
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

var cleanUpLineAnsiCode = "\r\033[K"
var moveToLineStartCode = "\r"
var dotPreLoaderSymbols = []string{".", "..", "..."}

type dotPreloaderCtrl struct {
	// message to show before preloader
	message string
	// interactiveMode: if true, preloader will be animated, otherwise it will be static
	interactiveMode    bool
	animationFrameRate time.Duration
	isActive           bool
	isActiveMu         sync.Mutex
	outputWriter       io.Writer
}

func (p *dotPreloaderCtrl) Start() {
	p.isActiveMu.Lock()
	p.isActive = true
	p.isActiveMu.Unlock()

	if p.interactiveMode {
		delay := p.animationFrameRate
		go func() {
			currentAnimationFrame := 0

			p.isActiveMu.Lock()
			for p.isActive {
				currentPreloaderSymbol := dotPreLoaderSymbols[currentAnimationFrame%len(dotPreLoaderSymbols)]

				fmt.Fprint(p.outputWriter, cleanUpLineAnsiCode, moveToLineStartCode, p.message, currentPreloaderSymbol)
				currentAnimationFrame++

				p.isActiveMu.Unlock()
				time.Sleep(delay)
				p.isActiveMu.Lock()
			}
			p.isActiveMu.Unlock()
		}()
	} else {
		fmt.Fprint(p.outputWriter, p.message, dotPreLoaderSymbols[len(dotPreLoaderSymbols)-1])
		fmt.Fprintln(p.outputWriter)
	}
}

func (p *dotPreloaderCtrl) StopWith(finishMessage string) {
	p.isActiveMu.Lock()
	p.isActive = false
	if p.interactiveMode {
		fmt.Fprint(p.outputWriter, cleanUpLineAnsiCode, moveToLineStartCode)
	}
	fmt.Fprint(p.outputWriter, finishMessage)

	p.isActiveMu.Unlock()
}

func (p *dotPreloaderCtrl) Stop() {
	p.StopWith("")
}

type DotPreloader interface {
	Start()
	Stop()
	StopWith(finishMessage string)
}

// NewDotPreloader factory to create new DotPreloader
// - message: message to show before preloader
// - interactiveMode: if true, preloader will be animated, otherwise it will be static
func NewDotPreloader(message string, interactiveMode bool) DotPreloader {
	return &dotPreloaderCtrl{
		message:            message,
		interactiveMode:    interactiveMode,
		animationFrameRate: 500 * time.Millisecond,
		outputWriter:       os.Stdout,
	}
}
