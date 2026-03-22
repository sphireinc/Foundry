//go:build !darwin && !linux

package service

import "time"

func processCPUTime() (time.Duration, time.Duration) {
	return 0, 0
}
