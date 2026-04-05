package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/backup"
	"github.com/sphireinc/foundry/internal/safepath"
)

func (s *Service) ListBackups(ctx context.Context) ([]types.BackupRecord, error) {
	_ = ctx
	items, err := backup.List(s.cfg.Backup.Dir)
	if err != nil {
		return nil, err
	}
	out := make([]types.BackupRecord, 0, len(items))
	for _, item := range items {
		out = append(out, backupRecord(item))
	}
	return out, nil
}

func (s *Service) CreateBackup(ctx context.Context, name string) (*types.BackupRecord, error) {
	_ = ctx
	name = strings.TrimSpace(name)
	var (
		snapshot *backup.Snapshot
		err      error
	)
	if name == "" {
		snapshot, err = backup.CreateManagedSnapshot(s.cfg)
	} else {
		validatedName, validateErr := validateBackupName(name)
		if validateErr != nil {
			return nil, validateErr
		}
		target, validateErr := safepath.ResolveRelativeUnderRoot(s.cfg.Backup.Dir, validatedName)
		if validateErr != nil {
			return nil, validateErr
		}
		snapshot, err = backup.CreateZipSnapshot(s.cfg, target)
	}
	if err != nil {
		return nil, err
	}
	record := backupRecord(*snapshot)
	return &record, nil
}

func (s *Service) RestoreBackup(ctx context.Context, name string) (*types.BackupRecord, error) {
	_ = ctx
	validatedName, err := validateBackupName(name)
	if err != nil {
		return nil, err
	}
	target, err := safepath.ResolveRelativeUnderRoot(s.cfg.Backup.Dir, validatedName)
	if err != nil {
		return nil, err
	}
	if err := backup.RestoreZipSnapshot(s.cfg, target); err != nil {
		return nil, err
	}
	s.invalidateGraphCache()
	info, err := os.Stat(target)
	if err != nil {
		return nil, err
	}
	return &types.BackupRecord{
		Name:      filepath.Base(target),
		Path:      target,
		SizeBytes: info.Size(),
		CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
	}, nil
}

func (s *Service) BackupPath(name string) (string, error) {
	validatedName, err := validateBackupName(name)
	if err != nil {
		return "", err
	}
	target, err := safepath.ResolveRelativeUnderRoot(s.cfg.Backup.Dir, validatedName)
	if err != nil {
		return "", err
	}
	if !backup.PathIsUnderBackupRoot(s.cfg, target) {
		return "", fmt.Errorf("backup path is outside backup root")
	}
	if _, err := os.Stat(target); err != nil {
		return "", err
	}
	return target, nil
}

func (s *Service) CreateGitBackupSnapshot(ctx context.Context, message string, push bool) (*types.BackupGitSnapshotRecord, error) {
	_ = ctx
	snapshot, err := backup.CreateGitSnapshot(s.cfg, message, push)
	if err != nil {
		return nil, err
	}
	return &types.BackupGitSnapshotRecord{
		RepoDir:   snapshot.RepoDir,
		Revision:  snapshot.Revision,
		CreatedAt: snapshot.CreatedAt.UTC().Format(time.RFC3339),
		Message:   snapshot.Message,
		Changed:   snapshot.Changed,
		Pushed:    snapshot.Pushed,
		RemoteURL: snapshot.RemoteURL,
		Branch:    snapshot.Branch,
	}, nil
}

func (s *Service) ListGitBackupSnapshots(ctx context.Context, limit int) ([]types.BackupGitSnapshotRecord, error) {
	_ = ctx
	items, err := backup.ListGitSnapshots(s.cfg, limit)
	if err != nil {
		return nil, err
	}
	out := make([]types.BackupGitSnapshotRecord, 0, len(items))
	for _, item := range items {
		out = append(out, types.BackupGitSnapshotRecord{
			RepoDir:   item.RepoDir,
			Revision:  item.Revision,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
			Message:   item.Message,
			Changed:   item.Changed,
			Pushed:    item.Pushed,
			RemoteURL: item.RemoteURL,
			Branch:    item.Branch,
		})
	}
	return out, nil
}

func backupRecord(item backup.Snapshot) types.BackupRecord {
	return types.BackupRecord{
		Name:      filepath.Base(item.Path),
		Path:      item.Path,
		SizeBytes: item.SizeBytes,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func validateBackupName(name string) (string, error) {
	validated, err := safepath.ValidatePathComponent("backup name", strings.TrimSpace(name))
	if err != nil {
		return "", err
	}
	return validated, nil
}
