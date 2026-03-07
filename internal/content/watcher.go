package content

import (
	"github.com/fsnotify/fsnotify"
)

func NewWatcher() (*fsnotify.Watcher, error) {
	return fsnotify.NewWatcher()
}
