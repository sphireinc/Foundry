package backup

import (
	"sync"
	"time"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/logx"
)

type AutoRunner struct {
	cfg      *config.Config
	mu       sync.Mutex
	timer    *time.Timer
	running  bool
	pending  bool
	debounce time.Duration
}

func NewAutoRunner(cfg *config.Config) *AutoRunner {
	if cfg == nil || !cfg.Backup.Enabled || !cfg.Backup.OnChange {
		return nil
	}
	return &AutoRunner{
		cfg:      cfg,
		debounce: time.Duration(cfg.Backup.DebounceSeconds) * time.Second,
	}
}

func (r *AutoRunner) Notify(path string) {
	if r == nil || !r.shouldTrack(path) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(r.debounce, r.run)
}

func (r *AutoRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func (r *AutoRunner) shouldTrack(path string) bool {
	if r == nil || path == "" {
		return false
	}
	if PathIsUnderBackupRoot(r.cfg, path) {
		return false
	}
	rel, err := filepathRelAbs(r.cfg.ContentDir, path)
	if err != nil {
		return false
	}
	_ = rel
	return true
}

func (r *AutoRunner) run() {
	r.mu.Lock()
	if r.running {
		r.pending = true
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	snapshot, err := CreateManagedSnapshot(r.cfg)
	if err != nil {
		logx.Warn("auto backup failed", "error", err)
	} else {
		logx.Info("auto backup created", "path", snapshot.Path, "size_bytes", snapshot.SizeBytes)
	}

	r.mu.Lock()
	r.running = false
	if r.pending {
		r.pending = false
		if r.timer != nil {
			r.timer.Stop()
		}
		r.timer = time.AfterFunc(r.debounce, r.run)
	}
	r.mu.Unlock()
}

func filepathRelAbs(root, target string) (string, error) {
	rootAbs, err := filepathAbs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepathAbs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepathRel(rootAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || stringsHasDotDotPrefix(rel) {
		return "", errOutsideRoot
	}
	return rel, nil
}
