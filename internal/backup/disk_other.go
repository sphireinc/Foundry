//go:build !darwin && !linux

package backup

import "fmt"

func freeSpace(path string) (uint64, error) {
	return 0, fmt.Errorf("free space check is not supported on this platform: %s", path)
}
