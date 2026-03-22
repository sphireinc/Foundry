package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"gopkg.in/yaml.v3"
)

const documentLockTTL = 2 * time.Minute

type documentLockRecord struct {
	SourcePath string    `yaml:"source_path"`
	Username   string    `yaml:"username"`
	Name       string    `yaml:"name,omitempty"`
	Role       string    `yaml:"role,omitempty"`
	Token      string    `yaml:"token"`
	LastBeatAt time.Time `yaml:"last_beat_at"`
	ExpiresAt  time.Time `yaml:"expires_at"`
}

type documentLockFile struct {
	Locks []documentLockRecord `yaml:"locks"`
}

func (s *Service) AcquireDocumentLock(ctx context.Context, req types.DocumentLockRequest) (*types.DocumentLockResponse, error) {
	identity, ok := currentIdentity(ctx)
	if !ok {
		return nil, fmt.Errorf("admin identity is required")
	}
	sourcePath, err := s.resolveContentPathAllowDerived(req.SourcePath)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	locks, err := s.loadLocksLocked(now)
	if err != nil {
		return nil, err
	}
	if existing, ok := locks[sourcePath]; ok {
		if existing.Token == strings.TrimSpace(req.LockToken) || strings.EqualFold(existing.Username, identity.Username) {
			existing.LastBeatAt = now
			existing.ExpiresAt = now.Add(documentLockTTL)
			locks[sourcePath] = existing
			if err := s.saveLocksLocked(locks); err != nil {
				return nil, err
			}
			return &types.DocumentLockResponse{Lock: toDocumentLock(existing, identity.Username, sourcePath, s.cfg.ContentDir)}, nil
		}
		return &types.DocumentLockResponse{Lock: toDocumentLock(existing, identity.Username, sourcePath, s.cfg.ContentDir)}, nil
	}

	token, err := randomLockToken()
	if err != nil {
		return nil, err
	}
	record := documentLockRecord{
		SourcePath: sourcePath,
		Username:   identity.Username,
		Name:       identity.Name,
		Role:       identity.Role,
		Token:      token,
		LastBeatAt: now,
		ExpiresAt:  now.Add(documentLockTTL),
	}
	locks[sourcePath] = record
	if err := s.saveLocksLocked(locks); err != nil {
		return nil, err
	}
	return &types.DocumentLockResponse{Lock: toDocumentLock(record, identity.Username, sourcePath, s.cfg.ContentDir)}, nil
}

func (s *Service) HeartbeatDocumentLock(ctx context.Context, req types.DocumentLockRequest) (*types.DocumentLockResponse, error) {
	identity, ok := currentIdentity(ctx)
	if !ok {
		return nil, fmt.Errorf("admin identity is required")
	}
	sourcePath, err := s.resolveContentPathAllowDerived(req.SourcePath)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	locks, err := s.loadLocksLocked(now)
	if err != nil {
		return nil, err
	}
	record, ok := locks[sourcePath]
	if !ok {
		return nil, fmt.Errorf("document lock not found")
	}
	if record.Token != strings.TrimSpace(req.LockToken) || !strings.EqualFold(record.Username, identity.Username) {
		return nil, fmt.Errorf("document is locked by another user")
	}
	record.LastBeatAt = now
	record.ExpiresAt = now.Add(documentLockTTL)
	locks[sourcePath] = record
	if err := s.saveLocksLocked(locks); err != nil {
		return nil, err
	}
	return &types.DocumentLockResponse{Lock: toDocumentLock(record, identity.Username, sourcePath, s.cfg.ContentDir)}, nil
}

func (s *Service) ReleaseDocumentLock(ctx context.Context, req types.DocumentLockRequest) error {
	identity, ok := currentIdentity(ctx)
	if !ok {
		return fmt.Errorf("admin identity is required")
	}
	sourcePath, err := s.resolveContentPathAllowDerived(req.SourcePath)
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	locks, err := s.loadLocksLocked(now)
	if err != nil {
		return err
	}
	record, ok := locks[sourcePath]
	if !ok {
		return nil
	}
	if record.Token != strings.TrimSpace(req.LockToken) && !strings.EqualFold(record.Username, identity.Username) && !adminauthCapabilityAllowed(identity, "users.manage") {
		return fmt.Errorf("document is locked by another user")
	}
	delete(locks, sourcePath)
	return s.saveLocksLocked(locks)
}

func (s *Service) DocumentLock(ctx context.Context, sourcePath string) (*types.DocumentLock, error) {
	identity, _ := currentIdentity(ctx)
	fullPath, err := s.resolveContentPathAllowDerived(sourcePath)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	locks, err := s.loadLocksLocked(now)
	if err != nil {
		return nil, err
	}
	record, ok := locks[fullPath]
	if !ok {
		return nil, nil
	}
	username := ""
	if identity != nil {
		username = identity.Username
	}
	lock := toDocumentLock(record, username, fullPath, s.cfg.ContentDir)
	return lock, nil
}

func (s *Service) ensureDocumentLock(ctx context.Context, sourcePath, lockToken string) error {
	identity, ok := currentIdentity(ctx)
	if !ok {
		return nil
	}
	if adminauthCapabilityAllowed(identity, "documents.write") || adminauthCapabilityAllowed(identity, "documents.review") || adminauthCapabilityAllowed(identity, "documents.lifecycle") {
		return nil
	}

	fullPath, err := s.resolveContentPathAllowDerived(sourcePath)
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	locks, err := s.loadLocksLocked(now)
	if err != nil {
		return err
	}
	record, ok := locks[fullPath]
	if !ok {
		return fmt.Errorf("document must be locked before saving")
	}
	if record.Token != strings.TrimSpace(lockToken) || !strings.EqualFold(record.Username, identity.Username) {
		return fmt.Errorf("document is locked by another user")
	}
	record.LastBeatAt = now
	record.ExpiresAt = now.Add(documentLockTTL)
	locks[fullPath] = record
	return s.saveLocksLocked(locks)
}

func (s *Service) loadLocksLocked(now time.Time) (map[string]documentLockRecord, error) {
	records := make(map[string]documentLockRecord)
	if strings.TrimSpace(s.cfg.Admin.LockFile) == "" {
		return records, nil
	}
	body, err := s.fs.ReadFile(s.cfg.Admin.LockFile)
	if err != nil {
		if os.IsNotExist(err) {
			return records, nil
		}
		return nil, err
	}
	var file documentLockFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		return nil, err
	}
	for _, record := range file.Locks {
		if record.SourcePath == "" || now.After(record.ExpiresAt) {
			continue
		}
		records[record.SourcePath] = record
	}
	return records, nil
}

func (s *Service) saveLocksLocked(locks map[string]documentLockRecord) error {
	entries := make([]documentLockRecord, 0, len(locks))
	for _, record := range locks {
		entries = append(entries, record)
	}
	body, err := yaml.Marshal(documentLockFile{Locks: entries})
	if err != nil {
		return err
	}
	if err := s.fs.MkdirAll(filepath.Dir(s.cfg.Admin.LockFile), 0o755); err != nil {
		return err
	}
	return s.fs.WriteFile(s.cfg.Admin.LockFile, body, 0o600)
}

func toDocumentLock(record documentLockRecord, currentUsername, sourcePath, contentDir string) *types.DocumentLock {
	expires := record.ExpiresAt
	lastBeat := record.LastBeatAt
	lock := &types.DocumentLock{
		SourcePath: displayDocumentPath(sourcePath, contentDir),
		Username:   record.Username,
		Name:       record.Name,
		Role:       record.Role,
		OwnedByMe:  strings.EqualFold(record.Username, currentUsername),
		Token:      record.Token,
		ExpiresAt:  &expires,
		LastBeatAt: &lastBeat,
	}
	if !lock.OwnedByMe {
		lock.Token = ""
	}
	return lock
}

func randomLockToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
