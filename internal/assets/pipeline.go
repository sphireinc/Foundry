package assets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

type Hooks interface {
	OnAssetsBuilding(*config.Config) error
}

type noopHooks struct{}

func (noopHooks) OnAssetsBuilding(*config.Config) error { return nil }

func Sync(cfg *config.Config, hooks Hooks) error {
	if hooks == nil {
		hooks = noopHooks{}
	}

	if err := hooks.OnAssetsBuilding(cfg); err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.PublicDir, 0o755); err != nil {
		return fmt.Errorf("create public dir: %w", err)
	}

	if cfg.Build.CopyAssets {
		src := filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir)
		dst := filepath.Join(cfg.PublicDir, "assets")
		if err := copyDirIfExists(src, dst); err != nil {
			return err
		}
	}

	if cfg.Build.CopyImages {
		src := filepath.Join(cfg.ContentDir, cfg.Content.ImagesDir)
		dst := filepath.Join(cfg.PublicDir, "images")
		if err := copyDirIfExists(src, dst); err != nil {
			return err
		}
	}

	if cfg.Build.CopyUploads {
		src := filepath.Join(cfg.ContentDir, cfg.Content.UploadsDir)
		dst := filepath.Join(cfg.PublicDir, "uploads")
		if err := copyDirIfExists(src, dst); err != nil {
			return err
		}
	}

	themeAssetsSrc := filepath.Join(cfg.ThemesDir, cfg.Theme, "assets")
	themeAssetsDst := filepath.Join(cfg.PublicDir, "theme")
	if err := copyDirIfExists(themeAssetsSrc, themeAssetsDst); err != nil {
		return err
	}

	pluginAssetsRoot := cfg.PluginsDir

	entries, err := os.ReadDir(pluginAssetsRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			src := filepath.Join(pluginAssetsRoot, entry.Name(), "assets")
			dst := filepath.Join(cfg.PublicDir, "plugins", entry.Name())

			if err := copyDirIfExists(src, dst); err != nil {
				return err
			}
		}
	}

	if err := buildCSSBundle(cfg); err != nil {
		return err
	}

	return nil
}

func buildCSSBundle(cfg *config.Config) error {
	themeCSSRoot := filepath.Join(cfg.ThemesDir, cfg.Theme, "assets", "css")
	contentCSSRoot := filepath.Join(cfg.ContentDir, cfg.Content.AssetsDir, "css")
	targetDir := filepath.Join(cfg.PublicDir, "assets", "css")
	targetFile := filepath.Join(targetDir, "foundry.bundle.css")

	files := make([]string, 0)

	themeFiles, err := listFiles(themeCSSRoot, ".css")
	if err != nil {
		return err
	}
	contentFiles, err := listFiles(contentCSSRoot, ".css")
	if err != nil {
		return err
	}

	files = append(files, themeFiles...)
	files = append(files, contentFiles...)

	if len(files) == 0 {
		return nil
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create css bundle dir: %w", err)
	}

	var sb strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read css file %s: %w", f, err)
		}
		sb.WriteString("/* ")
		sb.WriteString(filepath.ToSlash(f))
		sb.WriteString(" */\n")
		sb.Write(b)
		sb.WriteString("\n\n")
	}

	if err := os.WriteFile(targetFile, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write css bundle: %w", err)
	}

	return nil
}

func listFiles(root, ext string) ([]string, error) {
	out := make([]string, 0)

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return out, nil
	}

	err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ext) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files in %s: %w", root, err)
	}

	sort.Strings(out)
	return out, nil
}

func copyDirIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src file %s: %w", src, err)
	}
	defer func(in *os.File) {
		err := in.Close()
		if err != nil {
			_ = fmt.Errorf("close src file %s: %w", src, err)
		}
	}(in)

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir dst dir %s: %w", filepath.Dir(dst), err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("open dst file %s: %w", dst, err)
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			_ = fmt.Errorf("close dst file %s: %w", dst, err)
		}
	}(out)

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	return nil
}
