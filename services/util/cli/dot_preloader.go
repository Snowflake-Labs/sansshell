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
	"sync"
	"time"
)

var cleanUpLineAnsiCode = "\r\033[K"
var moveToLineStartCode = "\r"
var dotPreLoaderSymbols = []string{".", "..", "..."}

type dotPreloaderCtrl struct {
	message    string
	isActive   bool
	isActiveMu sync.Mutex
}

func (p *dotPreloaderCtrl) Start() {
	p.isActiveMu.Lock()
	p.isActive = true
	p.isActiveMu.Unlock()

	delay := 500 * time.Millisecond
	go func() {
		currentAnimationFrame := 0

		p.isActiveMu.Lock()
		for p.isActive {
			currentPreloaderSymbol := dotPreLoaderSymbols[currentAnimationFrame%len(dotPreLoaderSymbols)]
			fmt.Print(cleanUpLineAnsiCode, moveToLineStartCode, p.message, currentPreloaderSymbol)
			currentAnimationFrame++

			p.isActiveMu.Unlock()
			time.Sleep(delay)
			p.isActiveMu.Lock()
		}
		p.isActiveMu.Unlock()
	}()
}

func (p *dotPreloaderCtrl) StopWith(finishMessage string) {
	p.isActiveMu.Lock()
	p.isActive = false
	fmt.Print(cleanUpLineAnsiCode, moveToLineStartCode, finishMessage)
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

func NewDotPreloader(message string) DotPreloader {
	return &dotPreloaderCtrl{
		message: message,
	}
}
