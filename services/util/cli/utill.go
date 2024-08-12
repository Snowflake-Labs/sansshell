package cli

import (
	"fmt"
	"io"
)

var restoreFormatingCode = "\033[0m"
var RED_TEXT_COLOR = "\033[31m"
var GREEN_TEXT_COLOT = "\033[32m"
var YELLOW_TEXT_COLOR = "\033[33m"

func ColoredFprintf(w io.Writer, color string, format string, a ...any) (n int, err error) {
	line := fmt.Sprintf(format, a...)
	return fmt.Print(color, line, restoreFormatingCode)
}

func ColoredSprintf(color string, format string, a ...any) string {
	line := fmt.Sprintf(format, a...)
	return fmt.Sprint(color, line, restoreFormatingCode)
}

func ColoredSprint(color string, a ...any) string {
	line := fmt.Sprint(a...)
	return fmt.Sprint(color, line, restoreFormatingCode)
}

type ColorizedText struct {
	colorCode string
	text      string
}
