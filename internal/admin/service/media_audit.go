package service

import (
	"context"
	"github.com/sphireinc/foundry/internal/media"
)

func (s *Service) AuditMedia(ctx context.Context) (*media.AuditReport, error) {
	if err := requireCapability(ctx, "media.read"); err != nil {
		return nil, err
	}
	return media.Audit(s.cfg)
}
