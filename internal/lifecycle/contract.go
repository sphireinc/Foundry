package lifecycle

import "time"

type PathInfo struct {
	Path         string
	OriginalPath string
	State        State
	Timestamp    *time.Time
}

func DescribePath(path string) PathInfo {
	original, state, ts, ok := ParsePathDetails(path)
	if !ok {
		return PathInfo{
			Path:         path,
			OriginalPath: path,
			State:        StateCurrent,
		}
	}
	return PathInfo{
		Path:         path,
		OriginalPath: original,
		State:        state,
		Timestamp:    &ts,
	}
}
