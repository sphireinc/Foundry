package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/media"
	"github.com/sphireinc/foundry/internal/safepath"
	"gopkg.in/yaml.v3"
)

const maxMediaUploadSize = 256 << 20

func (s *Service) ListMedia(ctx context.Context) ([]types.MediaItem, error) {
	_ = ctx

	var items []types.MediaItem
	for _, collection := range []string{"images", "uploads", "assets"} {
		root, err := s.mediaRoot(collection)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(root); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if isMediaMetadataFile(path) || lifecycle.IsDerivedPath(path) {
				return nil
			}

			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			ref, err := media.NewReference(collection, filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			resolved, err := media.ResolveReference(ref)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			metadata, err := s.loadMediaMetadataFromPath(path)
			if err != nil {
				return err
			}

			items = append(items, types.MediaItem{
				Collection: collection,
				Path:       resolved.Path,
				Name:       filepath.Base(resolved.Path),
				Reference:  ref,
				PublicURL:  resolved.PublicURL,
				Kind:       string(resolved.Kind),
				Size:       info.Size(),
				Metadata:   metadata,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Collection != items[j].Collection {
			return items[i].Collection < items[j].Collection
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func (s *Service) GetMediaDetail(ctx context.Context, reference string) (*types.MediaDetailResponse, error) {
	_ = ctx

	item, path, err := s.resolveMediaItem(reference)
	if err != nil {
		return nil, err
	}
	metadata, err := s.loadMediaMetadataFromPath(path)
	if err != nil {
		return nil, err
	}
	item.Metadata = metadata
	usedBy, err := s.mediaUsage(reference)
	if err != nil {
		return nil, err
	}
	return &types.MediaDetailResponse{MediaItem: item, UsedBy: usedBy}, nil
}

func (s *Service) SaveMedia(ctx context.Context, collection, dir, filename, contentType string, body []byte) (*types.MediaUploadResponse, error) {
	_ = ctx

	if len(body) == 0 {
		return nil, fmt.Errorf("media upload body is required")
	}
	if len(body) > maxMediaUploadSize {
		return nil, fmt.Errorf("media upload exceeds %d bytes", maxMediaUploadSize)
	}

	collection = strings.TrimSpace(collection)
	if collection == "" {
		collection = media.DefaultCollection(filename, contentType)
	}
	root, err := s.mediaRoot(collection)
	if err != nil {
		return nil, err
	}

	cleanDir, err := cleanMediaDir(dir)
	if err != nil {
		return nil, err
	}

	safeName := media.SanitizeFilename(filename)
	targetDir := root
	relPrefix := ""
	if cleanDir != "" {
		targetDir, err = safepath.ResolveRelativeUnderRoot(root, filepath.FromSlash(cleanDir))
		if err != nil {
			return nil, err
		}
		relPrefix = cleanDir + "/"
	}

	if err := s.fs.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(targetDir, safeName)
	finalName := safeName
	created := true
	if _, err := s.fs.Stat(fullPath); err == nil {
		created = false
		if err := s.versionFile(fullPath, time.Now()); err != nil {
			return nil, err
		}
		if err := s.versionMediaMetadataForPrimary(fullPath, time.Now()); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := s.fs.WriteFile(fullPath, body, 0o644); err != nil {
		return nil, err
	}

	ref, err := media.NewReference(collection, relPrefix+finalName)
	if err != nil {
		return nil, err
	}
	resolved, err := media.ResolveReference(ref)
	if err != nil {
		return nil, err
	}

	return &types.MediaUploadResponse{
		MediaItem: types.MediaItem{
			Collection: collection,
			Path:       resolved.Path,
			Name:       finalName,
			Reference:  ref,
			PublicURL:  resolved.PublicURL,
			Kind:       string(resolved.Kind),
			Size:       int64(len(body)),
		},
		Created: created,
	}, nil
}

func (s *Service) SaveMediaMetadata(ctx context.Context, reference string, metadata types.MediaMetadata, versionComment, actor string) (*types.MediaDetailResponse, error) {
	_ = ctx

	item, path, err := s.resolveMediaItem(reference)
	if err != nil {
		return nil, err
	}
	metadata = normalizeMediaMetadata(metadata)
	sidecar := mediaMetadataPath(path)
	now := time.Now()
	if mediaMetadataEmpty(metadata) {
		if err := s.snapshotMediaMetadataVersion(path, now, versionComment, actor); err != nil {
			return nil, err
		}
		if err := s.fs.Remove(sidecar); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if err := s.snapshotMediaMetadataVersion(path, now, versionComment, actor); err != nil {
			return nil, err
		}
		body, err := yaml.Marshal(metadata)
		if err != nil {
			return nil, err
		}
		if err := s.fs.WriteFile(sidecar, body, 0o644); err != nil {
			return nil, err
		}
	}
	item.Metadata = metadata
	usedBy, err := s.mediaUsage(reference)
	if err != nil {
		return nil, err
	}
	return &types.MediaDetailResponse{MediaItem: item, UsedBy: usedBy}, nil
}

func (s *Service) mediaRoot(collection string) (string, error) {
	switch strings.TrimSpace(collection) {
	case "images":
		return filepath.Join(s.cfg.ContentDir, s.cfg.Content.ImagesDir), nil
	case "uploads":
		return filepath.Join(s.cfg.ContentDir, s.cfg.Content.UploadsDir), nil
	case "assets":
		return filepath.Join(s.cfg.ContentDir, s.cfg.Content.AssetsDir), nil
	default:
		return "", fmt.Errorf("unsupported media collection: %s", collection)
	}
}

func (s *Service) resolveMediaItem(reference string) (types.MediaItem, string, error) {
	ref, err := media.ResolveReference(reference)
	if err != nil {
		return types.MediaItem{}, "", err
	}
	root, err := s.mediaRoot(ref.Collection)
	if err != nil {
		return types.MediaItem{}, "", err
	}
	fullPath := filepath.Join(root, filepath.FromSlash(ref.Path))
	if err := ensureNoSymlinkEscape(root, fullPath); err != nil {
		return types.MediaItem{}, "", err
	}
	if lifecycle.IsDerivedPath(fullPath) {
		return types.MediaItem{}, "", fmt.Errorf("media reference must point to a current media file")
	}
	info, err := s.fs.Stat(fullPath)
	if err != nil {
		return types.MediaItem{}, "", err
	}
	return types.MediaItem{
		Collection: ref.Collection,
		Path:       ref.Path,
		Name:       filepath.Base(ref.Path),
		Reference:  reference,
		PublicURL:  ref.PublicURL,
		Kind:       string(ref.Kind),
		Size:       info.Size(),
	}, fullPath, nil
}

func (s *Service) loadMediaMetadataFromPath(path string) (types.MediaMetadata, error) {
	var metadata types.MediaMetadata
	body, err := s.fs.ReadFile(mediaMetadataPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return metadata, nil
		}
		return metadata, err
	}
	if err := yaml.Unmarshal(body, &metadata); err != nil {
		return metadata, err
	}
	return normalizeMediaMetadata(metadata), nil
}

func mediaMetadataPath(path string) string {
	return path + ".meta.yaml"
}

func mediaMetadataVersionPath(primaryPath string, now time.Time) string {
	return lifecycle.BuildVersionPath(primaryPath, now) + ".meta.yaml"
}

func mediaMetadataTrashPath(primaryPath string, now time.Time) string {
	return lifecycle.BuildTrashPath(primaryPath, now) + ".meta.yaml"
}

func isMediaMetadataFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".meta.yaml")
}

func normalizeMediaMetadata(metadata types.MediaMetadata) types.MediaMetadata {
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.Alt = strings.TrimSpace(metadata.Alt)
	metadata.Caption = strings.TrimSpace(metadata.Caption)
	metadata.Description = strings.TrimSpace(metadata.Description)
	metadata.Credit = strings.TrimSpace(metadata.Credit)
	if len(metadata.Tags) > 0 {
		tags := make([]string, 0, len(metadata.Tags))
		for _, tag := range metadata.Tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		metadata.Tags = tags
	}
	return metadata
}

type mediaMetadataDocument struct {
	types.MediaMetadata `yaml:",inline"`
	VersionComment      string `yaml:"version_comment,omitempty"`
	VersionedAt         string `yaml:"versioned_at,omitempty"`
	VersionActor        string `yaml:"version_actor,omitempty"`
}

func mediaMetadataEmpty(metadata types.MediaMetadata) bool {
	return metadata.Title == "" &&
		metadata.Alt == "" &&
		metadata.Caption == "" &&
		metadata.Description == "" &&
		metadata.Credit == "" &&
		len(metadata.Tags) == 0
}

func (s *Service) versionMediaMetadataForPrimary(primaryPath string, now time.Time) error {
	sidecar := mediaMetadataPath(primaryPath)
	if _, err := s.fs.Stat(sidecar); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	versionPath, err := s.uniqueDerivedPath(func(ts time.Time) string {
		return mediaMetadataVersionPath(primaryPath, ts)
	}, now)
	if err != nil {
		return err
	}
	if err := s.fs.Rename(sidecar, versionPath); err != nil {
		return err
	}
	return s.pruneVersions(sidecar)
}

func (s *Service) snapshotMediaMetadataVersion(primaryPath string, now time.Time, versionComment, actor string) error {
	sidecar := mediaMetadataPath(primaryPath)
	body, err := s.fs.ReadFile(sidecar)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	versionPath, err := s.uniqueDerivedPath(func(ts time.Time) string {
		return mediaMetadataVersionPath(primaryPath, ts)
	}, now)
	if err != nil {
		return err
	}
	if strings.TrimSpace(versionComment) == "" {
		if strings.TrimSpace(actor) == "" {
			if err := s.fs.WriteFile(versionPath, body, 0o644); err != nil {
				return err
			}
			return s.pruneVersions(sidecar)
		}
	}

	var metadataDoc mediaMetadataDocument
	if err := yaml.Unmarshal(body, &metadataDoc); err != nil {
		return err
	}
	metadataDoc.VersionComment = strings.TrimSpace(versionComment)
	metadataDoc.VersionedAt = now.UTC().Format(time.RFC3339)
	metadataDoc.VersionActor = strings.TrimSpace(actor)
	versionBody, err := yaml.Marshal(metadataDoc)
	if err != nil {
		return err
	}
	if err := s.fs.WriteFile(versionPath, versionBody, 0o644); err != nil {
		return err
	}
	return s.pruneVersions(sidecar)
}

func (s *Service) trashMediaMetadataForPrimary(primaryPath string, now time.Time) error {
	sidecar := mediaMetadataPath(primaryPath)
	if _, err := s.fs.Stat(sidecar); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	trashPath, err := s.uniqueDerivedPath(func(ts time.Time) string {
		return mediaMetadataTrashPath(primaryPath, ts)
	}, now)
	if err != nil {
		return err
	}
	return s.fs.Rename(sidecar, trashPath)
}

func cleanMediaDir(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	value = strings.Trim(value, "/")
	if value == "" {
		return "", nil
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == "/" || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid media dir: path must stay inside the media root")
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}
