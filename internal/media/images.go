package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/lifecycle"
	"github.com/sphireinc/foundry/internal/safepath"
	"golang.org/x/image/draw"
	"gopkg.in/yaml.v3"
)

const VariantDirectory = "_foundry/media"
const maxTransformBytes = 32 << 20
const maxTransformPixels = 40_000_000

type ImageMetadata struct {
	Alt        string `json:"alt,omitempty" yaml:"alt,omitempty"`
	Decorative bool   `json:"decorative,omitempty" yaml:"decorative,omitempty"`
}
type Variant struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type ImageEntry struct {
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Variants []Variant `json:"variants,omitempty"`
}
type ImageIndex map[string]ImageEntry

type Asset struct {
	Reference  string        `json:"reference"`
	PublicURL  string        `json:"public_url"`
	Kind       Kind          `json:"kind"`
	Size       int64         `json:"size"`
	SourcePath string        `json:"-"`
	Metadata   ImageMetadata `json:"metadata,omitempty"`
}

func CollectionRoot(cfg *config.Config, collection string) (string, error) {
	dirs := map[string]string{"images": cfg.Content.ImagesDir, "videos": cfg.Content.VideoDir, "audio": cfg.Content.AudioDir, "documents": cfg.Content.DocumentsDir, "uploads": cfg.Content.UploadsDir, "assets": cfg.Content.AssetsDir}
	dir, ok := dirs[collection]
	if !ok {
		return "", fmt.Errorf("unsupported media collection %q", collection)
	}
	root, err := safepath.ResolveRelativeUnderRoot(cfg.ContentDir, dir)
	if err != nil {
		return "", err
	}
	if err := safepath.EnsureNoSymlinkEscape(cfg.ContentDir, root); err != nil {
		return "", err
	}
	return root, nil
}

// Inventory excludes metadata, retained versions and trash, and rejects symlinks.
func Inventory(cfg *config.Config) ([]Asset, error) {
	var assets []Asset
	for _, collection := range SupportedCollections {
		root, err := CollectionRoot(cfg, collection)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(root, func(filename string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlinked media is not allowed: %s", filename)
			}
			if d.IsDir() {
				return nil
			}
			if lifecycle.IsDerivedPath(filename) || strings.HasSuffix(strings.ToLower(filename), ".meta.yaml") {
				return nil
			}
			if !d.Type().IsRegular() {
				return fmt.Errorf("media must be a regular file: %s", filename)
			}
			rel, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}
			reference, err := NewReference(collection, filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			resolved, err := ResolveReference(reference)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			metadata, err := ReadImageMetadata(root, filename)
			if err != nil {
				return err
			}
			assets = append(assets, Asset{Reference: reference, PublicURL: resolved.PublicURL, Kind: resolved.Kind, Size: info.Size(), SourcePath: filename, Metadata: metadata})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].Reference < assets[j].Reference })
	return assets, nil
}

func ReadImageMetadata(root, filename string) (ImageMetadata, error) {
	var metadata ImageMetadata
	sidecar := filename + ".meta.yaml"
	if err := safepath.EnsureNoSymlinkEscape(root, sidecar); err != nil {
		return metadata, err
	}
	// #nosec G304 -- caller supplies a contained media path; sidecar symlinks are rejected above.
	body, err := readRootFile(root, sidecar, 1<<20)
	if os.IsNotExist(err) {
		return metadata, nil
	}
	if err != nil {
		return metadata, err
	}
	err = yaml.Unmarshal(body, &metadata)
	metadata.Alt = strings.TrimSpace(metadata.Alt)
	return metadata, err
}

// BuildImages writes only derived output. Original media and sidecars remain intact.
func BuildImages(cfg *config.Config) error {
	if !cfg.Media.ResponsiveImages {
		return nil
	}
	options := cfg.Media
	if options.JPEGQuality == 0 {
		options.JPEGQuality = 82
	}
	if options.JPEGQuality < 1 || options.JPEGQuality > 100 {
		return fmt.Errorf("media.jpeg_quality must be between 1 and 100")
	}
	widths := append([]int(nil), options.Widths...)
	if len(widths) == 0 {
		widths = []int{320, 640, 960, 1280}
	}
	if len(widths) > 16 {
		return fmt.Errorf("media.widths supports at most 16 sizes")
	}
	for _, width := range widths {
		if width < 1 || width > 8192 {
			return fmt.Errorf("media.widths must be between 1 and 8192")
		}
	}
	sort.Ints(widths)
	inventory, err := Inventory(cfg)
	if err != nil {
		return err
	}
	index := ImageIndex{}
	for _, asset := range inventory {
		ref, _ := ResolveReference(asset.Reference)
		if !collectionCopied(cfg, ref.Collection) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(asset.SourcePath))
		// GIF animation, SVG, AVIF and WebP are preserved rather than flattened/transcoded.
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}
		if asset.Size > maxTransformBytes {
			return fmt.Errorf("image %s exceeds the 32 MiB transform limit", asset.Reference)
		}
		// #nosec G304 -- source is a regular file from the symlink-rejecting Inventory walk.
		root, err := CollectionRoot(cfg, ref.Collection)
		if err != nil {
			return err
		}
		body, err := readRootFile(root, asset.SourcePath, maxTransformBytes)
		if err != nil {
			return err
		}
		if ext == ".png" && animatedPNG(body) {
			continue
		}
		dimensions, format, err := image.DecodeConfig(bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("decode %s: %w", asset.Reference, err)
		}
		if dimensions.Width < 1 || dimensions.Height < 1 || int64(dimensions.Width)*int64(dimensions.Height) > maxTransformPixels {
			return fmt.Errorf("image %s exceeds the 40 megapixel transform limit", asset.Reference)
		}
		if format != "jpeg" && format != "png" {
			return fmt.Errorf("image format does not match transformable file %s", asset.Reference)
		}
		original, _, err := image.Decode(bytes.NewReader(body))
		if err != nil {
			return err
		}
		if format == "jpeg" {
			orientation := jpegOrientation(body)
			original = orientImage(original, orientation)
			dimensions.Width = original.Bounds().Dx()
			dimensions.Height = original.Bounds().Dy()
		}
		entry := ImageEntry{Width: dimensions.Width, Height: dimensions.Height}
		targets := append(append([]int(nil), widths...), dimensions.Width)
		sort.Ints(targets)
		last := 0
		for _, width := range targets {
			if width == last || width > dimensions.Width {
				continue
			}
			last = width
			height := max(1, int(int64(dimensions.Height)*int64(width)/int64(dimensions.Width)))
			output := original
			if width != dimensions.Width {
				resized := image.NewNRGBA(image.Rect(0, 0, width, height))
				draw.CatmullRom.Scale(resized, resized.Bounds(), original, original.Bounds(), draw.Src, nil)
				output = resized
			}
			var encoded bytes.Buffer
			suffix := ".png"
			if format == "jpeg" {
				suffix = ".jpg"
				err = jpeg.Encode(&encoded, output, &jpeg.Options{Quality: options.JPEGQuality})
			} else {
				err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&encoded, output)
			}
			if err != nil {
				return err
			}
			variantURL := (&url.URL{Path: asset.PublicURL}).EscapedPath()
			// Keep the original full-size bytes if re-encoding would make them larger.
			if width != dimensions.Width || encoded.Len() < len(body) {
				name := ContentHash([]byte(fmt.Sprintf("images-v1:%s:%d:%d", ContentHash(body), width, options.JPEGQuality))) + suffix
				relative := filepath.Join(VariantDirectory, name)
				if err := writeDerived(cfg.PublicDir, relative, encoded.Bytes()); err != nil {
					return err
				}
				variantURL = "/" + filepath.ToSlash(relative)
			}
			entry.Variants = append(entry.Variants, Variant{URL: variantURL, Width: width, Height: height})
		}
		index[asset.PublicURL] = entry
	}
	body, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return writeDerived(cfg.PublicDir, filepath.Join(VariantDirectory, "index.json"), body)
}

func collectionCopied(cfg *config.Config, collection string) bool {
	switch collection {
	case "images":
		return cfg.Build.CopyImages
	case "assets":
		return cfg.Build.CopyAssets
	default:
		return cfg.Build.CopyUploads
	}
}

func writeDerived(root, relative string, body []byte) error {
	filename, err := safepath.ResolveRelativeUnderRoot(root, relative)
	if err != nil {
		return err
	}
	if err = safepath.EnsureNoSymlinkEscape(root, filename); err != nil {
		return err
	}
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = scoped.Close() }()
	if err = scoped.MkdirAll(filepath.Dir(relative), 0o750); err != nil {
		return err
	}
	suffix, err := randomSuffix(12)
	if err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(relative), ".image-"+suffix)
	file, err := scoped.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = scoped.Remove(temporary) }()
	if _, err = file.Write(body); err != nil {
		_ = file.Close()
		return err
	}
	// #nosec G302 -- these are public static assets; descriptor-based chmod avoids path races.
	if err = file.Chmod(0o644); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return scoped.Rename(temporary, relative)
}

// readRootFile bounds both allocation and traversal, including concurrent source edits.
func readRootFile(root, filename string, limit int64) ([]byte, error) {
	relative, err := filepath.Rel(root, filename)
	if err != nil {
		return nil, err
	}
	scoped, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = scoped.Close() }()
	return readScopedFile(scoped, relative, limit)
}
func readScopedFile(scoped *os.Root, relative string, limit int64) ([]byte, error) {
	file, err := scoped.Open(relative)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("media source must be a regular file: %s", relative)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds read limit: %s", relative)
	}
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("file exceeds read limit: %s", relative)
	}
	return body, nil
}

func LoadImageIndex(cfg *config.Config) (ImageIndex, error) {
	if !cfg.Media.ResponsiveImages {
		return nil, nil
	}
	filename := filepath.Join(cfg.PublicDir, VariantDirectory, "index.json")
	if err := safepath.EnsureNoSymlinkEscape(cfg.PublicDir, filename); err != nil {
		return nil, err
	}
	// #nosec G304 -- fixed generated manifest beneath public_dir, checked for symlink escape.
	body, err := readRootFile(cfg.PublicDir, filename, 16<<20)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var index ImageIndex
	err = json.Unmarshal(body, &index)
	return index, err
}

func ImagePublicURL(value string) string {
	if strings.HasPrefix(value, ReferenceScheme) {
		if ref, err := ResolveReference(value); err == nil {
			return ref.PublicURL
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return ""
	}
	return parsed.Path
}
