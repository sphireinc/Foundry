//go:build !darwin && !linux

package server

import "time"

func processCPUTime() (time.Duration, time.Duration) {
	return 0, 0
}
