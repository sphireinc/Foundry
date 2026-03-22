//go:build darwin || linux

package service

import (
	"syscall"
	"time"
)

func processCPUTime() (time.Duration, time.Duration) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, 0
	}
	return timevalToDuration(usage.Utime), timevalToDuration(usage.Stime)
}

func timevalToDuration(tv syscall.Timeval) time.Duration {
	return time.Duration(tv.Sec)*time.Second + time.Duration(tv.Usec)*time.Microsecond
}
