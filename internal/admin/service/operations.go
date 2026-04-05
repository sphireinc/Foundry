package service

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/hostservice"
	"github.com/sphireinc/foundry/internal/logx"
	"github.com/sphireinc/foundry/internal/standalone"
	"github.com/sphireinc/foundry/internal/updater"
)

func (s *Service) GetOperationsStatus(ctx context.Context) (*types.OperationsStatusResponse, error) {
	_ = ctx
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	resp := &types.OperationsStatusResponse{}
	serviceStatus, err := hostservice.CheckStatus(projectDir)
	if err == nil && serviceStatus != nil {
		resp.ServiceInstalled = serviceStatus.Installed
		resp.ServiceRunning = serviceStatus.Running
		resp.ServiceEnabled = serviceStatus.Enabled
		resp.ServiceMessage = serviceStatus.Message
		if serviceStatus.Metadata != nil {
			resp.ServiceName = serviceStatus.Metadata.Name
			resp.ServiceFile = serviceStatus.Metadata.ServicePath
			resp.ServiceLog = serviceStatus.Metadata.LogPath
		}
	}
	if standaloneState, running, err := standalone.RunningState(projectDir); err == nil && standaloneState != nil {
		resp.StandalonePID = standaloneState.PID
		resp.StandaloneLog = standaloneState.LogPath
		resp.StandaloneActive = running
	}
	status, err := s.GetSystemStatus(context.Background())
	if err == nil && status != nil {
		resp.Checks = append([]types.HealthCheck(nil), status.Checks...)
	}
	return resp, nil
}

func (s *Service) ReadOperationsLog(ctx context.Context, lines int) (*types.OperationsLogResponse, error) {
	_ = ctx
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	serviceStatus, err := hostservice.CheckStatus(projectDir)
	if err == nil && serviceStatus != nil && serviceStatus.Metadata != nil && strings.TrimSpace(serviceStatus.Metadata.LogPath) != "" {
		content, readErr := standalone.ReadLastLines(serviceStatus.Metadata.LogPath, lines)
		if readErr == nil {
			return &types.OperationsLogResponse{
				Source:  "service",
				LogPath: serviceStatus.Metadata.LogPath,
				Content: content,
			}, nil
		}
	}
	if standaloneState, running, err := standalone.RunningState(projectDir); err == nil && standaloneState != nil && running {
		content, readErr := standalone.ReadLastLines(standaloneState.LogPath, lines)
		if readErr == nil {
			return &types.OperationsLogResponse{
				Source:  "standalone",
				LogPath: standaloneState.LogPath,
				Content: content,
			}, nil
		}
	}
	return &types.OperationsLogResponse{Source: "none", Content: ""}, nil
}

func (s *Service) ClearOperationalCaches(ctx context.Context) error {
	_ = ctx
	s.invalidateGraphCache()
	return nil
}

func (s *Service) RebuildSite(ctx context.Context) error {
	_ = ctx
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}
	executable, err := hostservice.EnsureExecutable(projectDir)
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, "build")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}
	s.invalidateGraphCache()
	return nil
}

func (s *Service) CheckForUpdates(ctx context.Context) (*types.UpdateStatusResponse, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	logx.Info("admin update check requested", "project_dir", projectDir)
	info, err := updater.Check(ctx, projectDir)
	if err != nil {
		return nil, err
	}
	logx.Info("admin update check completed", "current_version", info.CurrentVersion, "latest_version", info.LatestVersion, "has_update", info.HasUpdate, "install_mode", info.InstallMode, "apply_supported", info.ApplySupported)
	return &types.UpdateStatusResponse{
		Repo:                  info.Repo,
		CurrentVersion:        info.CurrentVersion,
		CurrentDisplayVersion: info.CurrentDisplayVersion,
		LatestVersion:         info.LatestVersion,
		HasUpdate:             info.HasUpdate,
		InstallMode:           string(info.InstallMode),
		ApplySupported:        info.ApplySupported,
		ReleaseURL:            info.ReleaseURL,
		PublishedAt:           info.PublishedAt.UTC().Format(time.RFC3339),
		Body:                  info.Body,
		AssetName:             info.AssetName,
		Instructions:          info.Instructions,
		NearestTag:            info.NearestTag,
		CurrentCommit:         info.CurrentCommit,
		Dirty:                 info.Dirty,
	}, nil
}

func (s *Service) ApplyUpdate(ctx context.Context) (*types.UpdateStatusResponse, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	logx.Info("admin update apply requested", "project_dir", projectDir)
	info, err := updater.ScheduleApply(ctx, projectDir)
	if err != nil {
		return nil, err
	}
	logx.Info("admin update apply scheduled", "current_version", info.CurrentVersion, "latest_version", info.LatestVersion, "install_mode", info.InstallMode, "asset_name", info.AssetName)
	return &types.UpdateStatusResponse{
		Repo:                  info.Repo,
		CurrentVersion:        info.CurrentVersion,
		CurrentDisplayVersion: info.CurrentDisplayVersion,
		LatestVersion:         info.LatestVersion,
		HasUpdate:             info.HasUpdate,
		InstallMode:           string(info.InstallMode),
		ApplySupported:        info.ApplySupported,
		ReleaseURL:            info.ReleaseURL,
		PublishedAt:           info.PublishedAt.UTC().Format(time.RFC3339),
		Body:                  info.Body,
		AssetName:             info.AssetName,
		Instructions:          info.Instructions,
		NearestTag:            info.NearestTag,
		CurrentCommit:         info.CurrentCommit,
		Dirty:                 info.Dirty,
	}, nil
}
