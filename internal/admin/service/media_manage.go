package service

import (
	"context"
	"os"
	"time"
)

func (s *Service) DeleteMedia(ctx context.Context, reference string) error {
	if err := requireCapability(ctx, "media.lifecycle"); err != nil {
		return err
	}
	_, fullPath, err := s.resolveMediaItem(reference)
	if err != nil {
		return err
	}
	now := time.Now()
	if _, err := s.trashFile(fullPath, now); err != nil {
		return err
	}
	if err := s.trashMediaMetadataForPrimary(fullPath, now); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
