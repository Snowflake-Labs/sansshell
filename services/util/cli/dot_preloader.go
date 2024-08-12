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
