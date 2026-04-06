package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/config"
)

func Log(cfg *config.Config, entry admintypes.AuditEntry) (logErr error) {
	if cfg == nil {
		return nil
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	path := filePath(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); logErr == nil && cerr != nil {
			logErr = cerr
		}
	}()

	enc := json.NewEncoder(f)
	logErr = enc.Encode(entry)
	return
}

func List(cfg *config.Config, limit int) ([]admintypes.AuditEntry, error) {
	if cfg == nil {
		return nil, nil
	}
	path := filePath(cfg)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []admintypes.AuditEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	items := make([]admintypes.AuditEntry, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry admintypes.AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		items = append(items, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func filePath(cfg *config.Config) string {
	if cfg == nil {
		return filepath.Join("data", "admin", "audit.jsonl")
	}
	return filepath.Join(cfg.DataDir, "admin", "audit.jsonl")
}
