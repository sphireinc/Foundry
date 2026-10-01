package media

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

func mediaFixture(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Config{ContentDir: filepath.Join(root, "content"), PublicDir: filepath.Join(root, "public"), DataDir: filepath.Join(root, "data"), ThemesDir: filepath.Join(root, "themes"), PluginsDir: filepath.Join(root, "plugins"), Theme: "default"}
	cfg.ApplyDefaults()
	cfg.Build.CopyImages = true
	for _, dir := range []string{cfg.ContentDir, cfg.PublicDir, cfg.DataDir, filepath.Join(cfg.ThemesDir, cfg.Theme), cfg.PluginsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	cfg.Media.ResponsiveImages = true
	cfg.Media.Widths = []int{40, 20, 20, 1000}
	return cfg
}
func writeFixture(t *testing.T, filename string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
func imageFixture(t *testing.T, cfg *config.Config) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 5), A: uint8(120 + x)})
		}
	}
	var body bytes.Buffer
	if err := png.Encode(&body, img); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "sample.png"), body.Bytes())
	return body.Bytes()
}
func TestResponsiveImagesDeterministicAndNonDestructive(t *testing.T) {
	cfg := mediaFixture(t)
	original := imageFixture(t, cfg)
	writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "sample.trash.20260101T000000Z.png"), []byte("not an image"))
	if err := BuildImages(cfg); err != nil {
		t.Fatal(err)
	}
	first, err := LoadImageIndex(cfg)
	if err != nil {
		t.Fatal(err)
	}
	entry := first["/images/sample.png"]
	if entry.Width != 80 || entry.Height != 40 || len(entry.Variants) != 3 {
		t.Fatalf("unexpected variants: %#v", entry)
	}
	generated := map[string][]byte{}
	for i, variant := range entry.Variants {
		if variant.Width != []int{20, 40, 80}[i] || variant.Height != variant.Width/2 {
			t.Fatalf("upscale, ordering or aspect ratio: %#v", variant)
		}
		if variant.URL == "/images/sample.png" {
			continue
		}
		filename := filepath.Join(cfg.PublicDir, filepath.FromSlash(strings.TrimPrefix(variant.URL, "/")))
		// #nosec G304 -- generated variant path returned by the test-owned builder.
		body, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		generated[filename] = body
		decoded, err := png.Decode(bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Bounds().Dx() != variant.Width || decoded.Bounds().Dy() != variant.Height {
			t.Fatal("variant dimensions differ from manifest")
		}
		_, _, _, alpha := decoded.At(0, 0).RGBA()
		if alpha == 65535 {
			t.Fatal("PNG transparency was lost")
		}
	}
	if err := BuildImages(cfg); err != nil {
		t.Fatal(err)
	}
	second, err := LoadImageIndex(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("manifest not deterministic")
	}
	for filename, expected := range generated {
		// #nosec G304 -- test-owned generated paths recorded above.
		actual, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(actual, expected) {
			t.Fatal("variant bytes changed")
		}
	}
	actual, err := os.ReadFile(filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "sample.png"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, original) {
		t.Fatal("original overwritten")
	}
}
func TestTransformsFailClosedOnSymlinksAndInvalidOptions(t *testing.T) {
	cfg := mediaFixture(t)
	imageFixture(t, cfg)
	cfg.Media.Widths = []int{-1}
	if err := BuildImages(cfg); err == nil {
		t.Fatal("invalid width accepted")
	}
	cfg.Media.Widths = []int{20}
	cfg.Media.JPEGQuality = 101
	if err := BuildImages(cfg); err == nil {
		t.Fatal("invalid quality accepted")
	}
	cfg.Media.JPEGQuality = 82
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(cfg.PublicDir, "_foundry")); err != nil {
		t.Fatal(err)
	}
	if err := BuildImages(cfg); err == nil {
		t.Fatal("output escape accepted")
	}
	if err := os.Remove(filepath.Join(cfg.PublicDir, "_foundry")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "escape")); err != nil {
		t.Fatal(err)
	}
	if _, err := Inventory(cfg); err == nil {
		t.Fatal("source symlink accepted")
	}
}
func TestEnrichHTMLAccessibilityAndExistingAttributes(t *testing.T) {
	cfg := mediaFixture(t)
	imageFixture(t, cfg)
	metadata := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "sample.png.meta.yaml")
	writeFixture(t, metadata, []byte("alt: 'A <green> tree'\n"))
	input := []byte(`<p>unchanged &amp; text</p><script>const x = '<img src="/images/sample.png">';</script><img src='/images/sample.png?version=1' alt='' width='300' srcset='custom.png 300w' sizes='50vw'>`)
	index := ImageIndex{"/images/sample.png": {Width: 80, Height: 40, Variants: []Variant{{URL: "/small.png", Width: 20, Height: 10}}}}
	result, err := EnrichHTML(input, cfg, index)
	if err != nil {
		t.Fatal(err)
	}
	text := string(result)
	for _, want := range []string{"<p>unchanged &amp; text</p>", "const x = '<img src=", `alt="A &lt;green&gt; tree"`, `srcset="custom.png 300w"`, `sizes="50vw"`, `width="300"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	if strings.Contains(text, `height=`) {
		t.Fatal("added incompatible height to author dimensions")
	}
	if issues := CheckHTML(result, "test"); len(issues) != 0 {
		t.Fatalf("valid alt flagged: %#v", issues)
	}
	writeFixture(t, metadata, []byte("decorative: true\n"))
	result, err = EnrichHTML([]byte(`<img src="/images/sample.png">`), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `alt=""`) || !strings.Contains(string(result), `role="presentation"`) {
		t.Fatalf("decorative attributes missing: %s", result)
	}
	if len(CheckHTML(result, "test")) != 0 {
		t.Fatal("decorative image flagged")
	}
	if len(CheckHTML([]byte(`<img src="a.png"><img src="b.png" alt=" "><img src="c.png" alt="description"><img src="d.png" alt="" role="presentation">`), "test")) != 2 {
		t.Fatal("missing/empty alt validation incorrect")
	}
}
func TestAuditReferencesAltAndLifecycleExclusions(t *testing.T) {
	cfg := mediaFixture(t)
	imageFixture(t, cfg)
	root := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir)
	writeFixture(t, filepath.Join(root, "unused.png"), []byte("original"))
	writeFixture(t, filepath.Join(root, "unused.png.meta.yaml"), []byte("alt: Not published\n"))
	writeFixture(t, filepath.Join(root, "old.trash.20260101T000000Z.png"), []byte("trash"))
	writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, "index.md"), []byte("---\ntitle: Home\n---\n![A tree](media:images/sample.png)\n![](https://example.com/no-alt.png)"))
	report, err := Audit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.PotentialOrphans, []string{"media:images/unused.png"}) {
		t.Fatalf("unsafe orphan preview: %#v", report.PotentialOrphans)
	}
	if len(report.Accessibility) != 1 || report.Accessibility[0].Image != "https://example.com/no-alt.png" {
		t.Fatalf("incorrect accessibility: %#v", report.Accessibility)
	}
	if len(report.Assets) != 2 {
		t.Fatal("sidecar/trash listed as current media")
	}
	// Public URLs in theme CSS protect assets too, including URL-escaped names.
	writeFixture(t, filepath.Join(cfg.ThemesDir, cfg.Theme, "assets", "style.css"), []byte(`body { background: url('/images/unused.png?v=1') }`))
	report, err = Audit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.PotentialOrphans) != 0 {
		t.Fatal("theme reference ignored")
	}
	if _, err := os.Stat(filepath.Join(root, "unused.png")); err != nil {
		t.Fatal("audit mutated originals")
	}
}

func TestAuditProtectsImagesUsedByContentAssets(t *testing.T) {
	cfg := mediaFixture(t)
	imageFixture(t, cfg)
	writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "css", "site.css"), []byte(`body { background-image: url("/images/sample.png") }`))
	report, err := Audit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(report.PotentialOrphans, "media:images/sample.png") {
		t.Fatal("image referenced by content CSS classified as orphan")
	}
}

func TestAuditHandlesEncodedURLsAlongsideCSSPercentages(t *testing.T) {
	cfg := mediaFixture(t)
	writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "with space.png"), []byte("original"))
	writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "site.css"), []byte(`body { width: 100%; background: url("/images/with%20space.png") }`))
	report, err := Audit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(report.PotentialOrphans, "media:images/with space.png") {
		t.Fatal("encoded reference hidden by unrelated percentage")
	}
}

func TestAuditDoesNotParseArchetypesAsPublishedDocuments(t *testing.T) {
	cfg := mediaFixture(t)
	writeFixture(t, filepath.Join(cfg.ContentDir, "archetypes", "page.md"), []byte("---\ntitle: {{title}}\n---\n![](media:images/template.png)"))
	report, err := Audit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Accessibility) != 0 {
		t.Fatal("unexpanded archetype reported as published image")
	}
}

func TestImagePixelLimitAndAnimatedPNGPreservation(t *testing.T) {
	cfg := mediaFixture(t)
	original := imageFixture(t, cfg)
	oversized := append([]byte(nil), original...)
	binary.BigEndian.PutUint32(oversized[16:20], 50000)
	binary.BigEndian.PutUint32(oversized[20:24], 1000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	filename := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir, "sample.png")
	writeFixture(t, filename, oversized)
	if err := BuildImages(cfg); err == nil || !strings.Contains(err.Error(), "40 megapixel") {
		t.Fatalf("pixel limit not enforced before decoding: %v", err)
	}
	control := make([]byte, 20)
	binary.BigEndian.PutUint32(control[:4], 8)
	copy(control[4:8], "acTL")
	binary.BigEndian.PutUint32(control[8:12], 2)
	binary.BigEndian.PutUint32(control[16:20], crc32.ChecksumIEEE(control[4:16]))
	animated := append(append(append([]byte(nil), original[:33]...), control...), original[33:]...)
	writeFixture(t, filename, animated)
	if err := BuildImages(cfg); err != nil {
		t.Fatal(err)
	}
	index, err := LoadImageIndex(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 0 {
		t.Fatal("animated PNG was flattened into a static variant")
	}
	// #nosec G304 -- test-owned original verifies all animated bytes were preserved.
	body, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, animated) {
		t.Fatal("animated source overwritten")
	}
}

func TestAuditProtectsImplicitCSSBundleInputs(t *testing.T) {
	cfg := mediaFixture(t)
	cfg.Content.AssetsDir = "custom-assets"
	cfg.Build.CopyAssets = false
	for _, name := range []string{"css/site.css", "css/nested/other.CSS", "unused.css"} {
		writeFixture(t, filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, filepath.FromSlash(name)), []byte("body { color: red }"))
	}
	report, err := Audit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.PotentialOrphans, []string{"media:assets/unused.css"}) {
		t.Fatalf("incorrect CSS orphan candidates: %#v", report.PotentialOrphans)
	}
	for _, asset := range report.Assets {
		if strings.Contains(asset.Reference, "/css/") && !slices.Contains(asset.UsedBy, "generated /assets/css/foundry.bundle.css") {
			t.Fatalf("missing implicit dependency: %#v", asset)
		}
	}
}

func TestGeneratedDirectoriesArePubliclyTraversable(t *testing.T) {
	cfg := mediaFixture(t)
	imageFixture(t, cfg)
	// Simulate the permissions left by an older build.
	for _, directory := range []string{"_foundry", VariantDirectory} {
		if err := os.MkdirAll(filepath.Join(cfg.PublicDir, directory), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := BuildImages(cfg); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"_foundry", VariantDirectory} {
		info, err := os.Stat(filepath.Join(cfg.PublicDir, directory))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("public directory %s has permissions %o", directory, info.Mode().Perm())
		}
	}
}
