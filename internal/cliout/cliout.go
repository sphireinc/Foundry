package cliout

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

var (
	headingColor = color.New(color.Bold, color.FgCyan)
	labelColor   = color.New(color.Bold, color.FgBlue)
	okColor      = color.New(color.Bold, color.FgGreen)
	warnColor    = color.New(color.Bold, color.FgYellow)
	failColor    = color.New(color.Bold, color.FgRed)
	mutedColor   = color.New(color.Faint)
)

func Heading(text string) string {
	return headingColor.Sprint(text)
}

func Label(text string) string {
	return labelColor.Sprint(text)
}

func OK(text string) string {
	return okColor.Sprint(text)
}

func Warning(text string) string {
	return warnColor.Sprint(text)
}

func Fail(text string) string {
	return failColor.Sprint(text)
}

func Muted(text string) string {
	return mutedColor.Sprint(text)
}

func Successf(format string, args ...any) {
	_, err := fmt.Fprintln(color.Output, OK(fmt.Sprintf(format, args...)))
	if err != nil {
		return // TODO Handle this error at some point, even if redundant
	}
}

func Errorf(format string, args ...any) {
	_, err := fmt.Fprintln(color.Error, Fail(fmt.Sprintf(format, args...)))
	if err != nil {
		return // TODO Handle this error at some point, even if redundant
	}
}

func Println(text string) {
	_, err := fmt.Fprintln(color.Output, text)
	if err != nil {
		return // TODO Handle this error at some point, even if redundant
	}
}

func Stderr(text string) {
	_, err := fmt.Fprintln(os.Stderr, text)
	if err != nil {
		return // TODO Handle this error at some point, even if redundant
	}
}

func StatusLabel(ok bool) string {
	if ok {
		return OK("OK")
	}
	return Fail("FAIL")
}
