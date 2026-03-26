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

func (s *Service) ListMedia(ctx context.Context, query ...string) ([]types.MediaItem, error) {
	if err := requireCapability(ctx, "media.read"); err != nil {
		return nil, err
	}
	search := ""
	if len(query) > 0 {
		search = strings.ToLower(strings.TrimSpace(query[0]))
	}

	var items []types.MediaItem
	for _, collection := range []string{"images", "videos", "audio", "documents", "uploads", "assets"} {
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
			if search != "" && !matchesMediaQuery(resolved.Path, ref, metadata, search) {
				return nil
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
	if err := requireCapability(ctx, "media.read"); err != nil {
		return nil, err
	}

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
	if err := requireCapability(ctx, "media.write"); err != nil {
		return nil, err
	}
	_ = contentType

	if len(body) == 0 {
		return nil, fmt.Errorf("media upload body is required")
	}
	if len(body) > maxMediaUploadSize {
		return nil, fmt.Errorf("media upload exceeds %d bytes", maxMediaUploadSize)
	}

	now := time.Now()
	uploadInfo, err := media.PrepareUpload(collection, filename, body, now)
	if err != nil {
		return nil, err
	}
	root, err := s.mediaRoot(uploadInfo.Collection)
	if err != nil {
		return nil, err
	}

	cleanDir, err := s.cleanMediaDir(uploadInfo.Collection, dir)
	if err != nil {
		return nil, err
	}

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

	fullPath := filepath.Join(targetDir, uploadInfo.StoredFilename)
	if err := ensureNoSymlinkEscape(root, fullPath); err != nil {
		return nil, err
	}
	finalName := uploadInfo.StoredFilename
	created := true
	if _, err := s.fs.Stat(fullPath); err == nil {
		return nil, fmt.Errorf("generated media filename collision; retry upload")
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := s.fs.WriteFile(fullPath, body, 0o644); err != nil {
		return nil, err
	}
	if err := s.writeUploadedMediaMetadata(fullPath, uploadInfo, actorLabelFromContext(ctx), now); err != nil {
		return nil, err
	}

	ref, err := media.NewReference(uploadInfo.Collection, relPrefix+finalName)
	if err != nil {
		return nil, err
	}
	resolved, err := media.ResolveReference(ref)
	if err != nil {
		return nil, err
	}

	return &types.MediaUploadResponse{
		MediaItem: types.MediaItem{
			Collection: uploadInfo.Collection,
			Path:       resolved.Path,
			Name:       finalName,
			Reference:  ref,
			PublicURL:  resolved.PublicURL,
			Kind:       string(uploadInfo.Kind),
			Size:       uploadInfo.Size,
			Metadata: types.MediaMetadata{
				Title:            uploadTitle(uploadInfo.OriginalFilename),
				OriginalFilename: uploadInfo.OriginalFilename,
				StoredFilename:   uploadInfo.StoredFilename,
				Extension:        uploadInfo.Extension,
				MIMEType:         uploadInfo.MIMEType,
				Kind:             string(uploadInfo.Kind),
				ContentHash:      uploadInfo.ContentHash,
				FileSize:         uploadInfo.Size,
				UploadedAt:       timePtr(now.UTC()),
				UploadedBy:       actorLabelFromContext(ctx),
			},
		},
		Created: created,
	}, nil
}

func (s *Service) ReplaceMedia(ctx context.Context, reference, contentType string, body []byte) (*types.MediaReplaceResponse, error) {
	if err := requireCapability(ctx, "media.write"); err != nil {
		return nil, err
	}
	_ = contentType

	if len(body) == 0 {
		return nil, fmt.Errorf("media upload body is required")
	}
	if len(body) > maxMediaUploadSize {
		return nil, fmt.Errorf("media upload exceeds %d bytes", maxMediaUploadSize)
	}

	item, fullPath, err := s.resolveMediaItem(reference)
	if err != nil {
		return nil, err
	}
	root, err := s.mediaRoot(item.Collection)
	if err != nil {
		return nil, err
	}
	if err := ensureNoSymlinkEscape(root, fullPath); err != nil {
		return nil, err
	}
	now := time.Now()
	uploadInfo, err := media.PrepareUpload(item.Collection, item.Name, body, now)
	if err != nil {
		return nil, err
	}

	if err := s.versionFile(fullPath, now); err != nil {
		return nil, err
	}
	if err := s.versionMediaMetadataForPrimary(fullPath, now); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err := s.fs.WriteFile(fullPath, body, 0o644); err != nil {
		return nil, err
	}
	currentMetadata, err := s.loadMediaMetadataFromPath(fullPath)
	if err != nil {
		return nil, err
	}
	uploadInfo.OriginalFilename = firstNonEmptyMedia(currentMetadata.OriginalFilename, item.Name)
	uploadInfo.StoredFilename = item.Name
	uploadInfo.SafeFilename = item.Name
	mergedMetadata := preserveEditableMediaMetadata(currentMetadata, uploadInfo, actorLabelFromContext(ctx), now)
	if err := s.writeMediaMetadataDocument(fullPath, mergedMetadata); err != nil {
		return nil, err
	}

	info, err := s.fs.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	item.Size = info.Size()
	item.Kind = string(uploadInfo.Kind)
	item.Metadata = mergedMetadata

	return &types.MediaReplaceResponse{
		MediaItem: item,
		Replaced:  true,
	}, nil
}

func (s *Service) SaveMediaMetadata(ctx context.Context, reference string, metadata types.MediaMetadata, versionComment, actor string) (*types.MediaDetailResponse, error) {
	if err := requireCapability(ctx, "media.write"); err != nil {
		return nil, err
	}

	item, path, err := s.resolveMediaItem(reference)
	if err != nil {
		return nil, err
	}
	existingMetadata, err := s.loadMediaMetadataFromPath(path)
	if err != nil {
		return nil, err
	}
	metadata = mergeEditableMediaMetadata(existingMetadata, metadata)
	sidecar, err := s.mediaSidecarPath(path)
	if err != nil {
		return nil, err
	}
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

func matchesMediaQuery(pathValue, reference string, metadata types.MediaMetadata, query string) bool {
	for _, candidate := range []string{
		pathValue,
		reference,
		metadata.Title,
		metadata.Alt,
		metadata.Caption,
		metadata.Description,
		metadata.Credit,
		metadata.OriginalFilename,
		metadata.StoredFilename,
		metadata.MIMEType,
		metadata.Kind,
		metadata.ContentHash,
		metadata.UploadedBy,
	} {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	for _, tag := range metadata.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

func (s *Service) collectionDir(collection string) (string, error) {
	switch strings.TrimSpace(collection) {
	case "images":
		return s.cfg.Content.ImagesDir, nil
	case "videos":
		return s.cfg.Content.VideoDir, nil
	case "audio":
		return s.cfg.Content.AudioDir, nil
	case "documents":
		return s.cfg.Content.DocumentsDir, nil
	case "uploads":
		return s.cfg.Content.UploadsDir, nil
	case "assets":
		return s.cfg.Content.AssetsDir, nil
	default:
		return "", fmt.Errorf("unsupported media collection: %s", collection)
	}
}

func (s *Service) mediaRoot(collection string) (string, error) {
	collectionDir, err := s.collectionDir(collection)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.cfg.ContentDir, collectionDir), nil
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
	sidecar, err := s.mediaSidecarPath(path)
	if err != nil {
		return metadata, err
	}
	body, err := s.fs.ReadFile(sidecar)
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
	metadata.OriginalFilename = strings.TrimSpace(metadata.OriginalFilename)
	metadata.StoredFilename = strings.TrimSpace(metadata.StoredFilename)
	metadata.Extension = strings.ToLower(strings.TrimSpace(metadata.Extension))
	metadata.MIMEType = strings.ToLower(strings.TrimSpace(metadata.MIMEType))
	metadata.Kind = strings.TrimSpace(metadata.Kind)
	metadata.ContentHash = strings.ToLower(strings.TrimSpace(metadata.ContentHash))
	metadata.FocalX = strings.TrimSpace(metadata.FocalX)
	metadata.FocalY = strings.TrimSpace(metadata.FocalY)
	metadata.UploadedBy = strings.TrimSpace(metadata.UploadedBy)
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
		len(metadata.Tags) == 0 &&
		metadata.OriginalFilename == "" &&
		metadata.StoredFilename == "" &&
		metadata.Extension == "" &&
		metadata.MIMEType == "" &&
		metadata.Kind == "" &&
		metadata.ContentHash == "" &&
		metadata.FileSize == 0 &&
		metadata.FocalX == "" &&
		metadata.FocalY == "" &&
		metadata.UploadedAt == nil &&
		metadata.UploadedBy == ""
}

func (s *Service) versionMediaMetadataForPrimary(primaryPath string, now time.Time) error {
	sidecar, err := s.mediaSidecarPath(primaryPath)
	if err != nil {
		return err
	}
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
	sidecar, err := s.mediaSidecarPath(primaryPath)
	if err != nil {
		return err
	}
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
	sidecar, err := s.mediaSidecarPath(primaryPath)
	if err != nil {
		return err
	}
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

func (s *Service) mediaSidecarPath(primaryPath string) (string, error) {
	_, _, resolved, err := s.mediaReferenceInfoForPath(primaryPath)
	if err != nil {
		return "", err
	}
	root, err := s.mediaRoot(resolved.Collection)
	if err != nil {
		return "", err
	}
	sidecar := mediaMetadataPath(primaryPath)
	if err := ensureNoSymlinkEscape(root, sidecar); err != nil {
		return "", err
	}
	return sidecar, nil
}

func (s *Service) writeUploadedMediaMetadata(primaryPath string, info media.UploadInfo, actor string, now time.Time) error {
	metadata := types.MediaMetadata{
		Title:            uploadTitle(info.OriginalFilename),
		OriginalFilename: info.OriginalFilename,
		StoredFilename:   info.StoredFilename,
		Extension:        info.Extension,
		MIMEType:         info.MIMEType,
		Kind:             string(info.Kind),
		ContentHash:      info.ContentHash,
		FileSize:         info.Size,
		UploadedAt:       timePtr(now.UTC()),
		UploadedBy:       strings.TrimSpace(actor),
	}
	return s.writeMediaMetadataDocument(primaryPath, metadata)
}

func (s *Service) writeMediaMetadataDocument(primaryPath string, metadata types.MediaMetadata) error {
	sidecar, err := s.mediaSidecarPath(primaryPath)
	if err != nil {
		return err
	}
	body, err := yaml.Marshal(normalizeMediaMetadata(metadata))
	if err != nil {
		return err
	}
	return s.fs.WriteFile(sidecar, body, 0o644)
}

func preserveEditableMediaMetadata(existing types.MediaMetadata, info media.UploadInfo, actor string, now time.Time) types.MediaMetadata {
	existing = normalizeMediaMetadata(existing)
	existing.OriginalFilename = info.OriginalFilename
	existing.StoredFilename = info.StoredFilename
	existing.Extension = info.Extension
	existing.MIMEType = info.MIMEType
	existing.Kind = string(info.Kind)
	existing.ContentHash = info.ContentHash
	existing.FileSize = info.Size
	if existing.UploadedAt == nil {
		existing.UploadedAt = timePtr(now.UTC())
	}
	if strings.TrimSpace(existing.UploadedBy) == "" {
		existing.UploadedBy = strings.TrimSpace(actor)
	}
	if strings.TrimSpace(existing.Title) == "" {
		existing.Title = uploadTitle(info.OriginalFilename)
	}
	return existing
}

func mergeEditableMediaMetadata(existing, requested types.MediaMetadata) types.MediaMetadata {
	existing = normalizeMediaMetadata(existing)
	requested = normalizeMediaMetadata(requested)
	existing.Title = requested.Title
	existing.Alt = requested.Alt
	existing.Caption = requested.Caption
	existing.Description = requested.Description
	existing.Credit = requested.Credit
	existing.Tags = append([]string(nil), requested.Tags...)
	existing.FocalX = requested.FocalX
	existing.FocalY = requested.FocalY
	return existing
}

func uploadTitle(filename string) string {
	name := strings.TrimSuffix(filepath.Base(strings.TrimSpace(filename)), filepath.Ext(strings.TrimSpace(filename)))
	name = strings.TrimSpace(strings.ReplaceAll(name, "-", " "))
	name = strings.TrimSpace(strings.ReplaceAll(name, "_", " "))
	if name == "" {
		return "Upload"
	}
	return name
}

func firstNonEmptyMedia(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func actorLabelFromContext(ctx context.Context) string {
	if identity, ok := currentIdentity(ctx); ok {
		if strings.TrimSpace(identity.Name) != "" {
			return strings.TrimSpace(identity.Name)
		}
		return strings.TrimSpace(identity.Username)
	}
	return ""
}

func timePtr(v time.Time) *time.Time {
	return &v
}

func (s *Service) cleanMediaDir(collection, value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, `\`, "/"))
	value = strings.Trim(value, "/")
	if value == "" {
		return "", nil
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == "/" || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid media dir: path must stay inside the media root")
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	contentPrefix := strings.Trim(strings.ReplaceAll(s.cfg.ContentDir, `\`, "/"), "/")
	contentBase := strings.Trim(filepath.Base(s.cfg.ContentDir), "/")
	collectionDir, err := s.collectionDir(collection)
	if err != nil {
		return "", err
	}
	collectionPrefix := strings.Trim(strings.ReplaceAll(collectionDir, `\`, "/"), "/")
	for _, prefix := range []string{
		collectionPrefix,
		path.Join(contentBase, collectionPrefix),
		path.Join(contentPrefix, collectionPrefix),
	} {
		prefix = strings.Trim(prefix, "/")
		if prefix == "" {
			continue
		}
		if cleaned == prefix {
			return "", nil
		}
		if strings.HasPrefix(cleaned, prefix+"/") {
			cleaned = strings.TrimPrefix(cleaned, prefix+"/")
			break
		}
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}
