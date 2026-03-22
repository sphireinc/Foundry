package contentcmd

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/content"
	"github.com/sphireinc/foundry/internal/site"
	"gopkg.in/yaml.v3"
)

type command struct{}

type contentRow struct {
	Type   string
	Lang   string
	Title  string
	Slug   string
	URL    string
	Draft  bool
	Source string
}

func (command) Name() string {
	return "content"
}

func (command) Summary() string {
	return "Manage and inspect content"
}

func (command) Group() string {
	return "content commands"
}

func (command) Details() []string {
	return []string{
		"foundry content lint",
		"foundry content new page <slug>",
		"foundry content new post <slug>",
		"foundry content list",
		"foundry content graph",
		"foundry content export <bundle.zip>",
		"foundry content import markdown <dir>",
		"foundry content import wordpress <wxr.xml>",
		"foundry content migrate layout <from> <to>",
		"foundry content migrate field-rename <schema> <old> <new>",
	}
}

func (command) RequiresConfig() bool {
	return true
}

func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry content [lint|new|list|graph|export|import|migrate]")
	}

	switch args[2] {
	case "lint":
		return runLint(cfg)
	case "new":
		return runNew(cfg, args)
	case "list":
		return runList(cfg)
	case "graph":
		return runGraph(cfg)
	case "export":
		return runExport(cfg, args)
	case "import":
		return runImport(cfg, args)
	case "migrate":
		return runMigrate(cfg, args)
	default:
		return fmt.Errorf("unknown content subcommand: %s", args[2])
	}
}

func runLint(cfg *config.Config) error {
	graph, err := loadGraph(cfg, true)
	if err != nil {
		return err
	}

	errCount := 0
	seenSource := make(map[string]struct{})
	seenSlugByTypeLang := make(map[string]string)

	for _, doc := range graph.Documents {
		if strings.TrimSpace(doc.Title) == "" {
			fmt.Printf("missing title: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Slug) == "" {
			fmt.Printf("missing slug: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Layout) == "" {
			fmt.Printf("missing layout: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Type) == "" {
			fmt.Printf("missing type: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.Lang) == "" {
			fmt.Printf("missing lang: %s\n", doc.SourcePath)
			errCount++
		}
		if strings.TrimSpace(doc.URL) == "" {
			fmt.Printf("missing URL: %s\n", doc.SourcePath)
			errCount++
		}

		if _, ok := seenSource[doc.SourcePath]; ok {
			fmt.Printf("duplicate source path: %s\n", doc.SourcePath)
			errCount++
		}
		seenSource[doc.SourcePath] = struct{}{}

		key := doc.Type + "|" + doc.Lang + "|" + doc.Slug
		if other, ok := seenSlugByTypeLang[key]; ok {
			fmt.Printf("duplicate slug within type/lang %q for %s and %s\n", key, other, doc.SourcePath)
			errCount++
		} else {
			seenSlugByTypeLang[key] = doc.SourcePath
		}
	}

	if errCount > 0 {
		return fmt.Errorf("content lint failed with %d error(s)", errCount)
	}

	fmt.Printf("content lint OK (%d document(s))\n", len(graph.Documents))
	return nil
}

func runNew(cfg *config.Config, args []string) error {
	if len(args) < 5 {
		return fmt.Errorf("usage: foundry content new [page|post] <slug>")
	}

	kind := strings.TrimSpace(args[3])
	slug := normalizeSlug(args[4])
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}

	var path string
	switch kind {
	case "page":
		path = filepath.Join(cfg.ContentDir, cfg.Content.PagesDir, slug+".md")
	case "post":
		path = filepath.Join(cfg.ContentDir, cfg.Content.PostsDir, slug+".md")
	default:
		return fmt.Errorf("unknown content type: %s", kind)
	}

	body, err := content.BuildNewContent(cfg, kind, slug)
	if err != nil {
		return err
	}

	return writeNewContentFile(path, body)
}

func runList(cfg *config.Config) error {
	graph, err := loadGraph(cfg, true)
	if err != nil {
		return err
	}

	rows := make([]contentRow, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		rows = append(rows, contentRow{
			Type:   doc.Type,
			Lang:   doc.Lang,
			Title:  doc.Title,
			Slug:   doc.Slug,
			URL:    doc.URL,
			Draft:  doc.Draft,
			Source: doc.SourcePath,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type != rows[j].Type {
			return rows[i].Type < rows[j].Type
		}
		if rows[i].Lang != rows[j].Lang {
			return rows[i].Lang < rows[j].Lang
		}
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		return rows[i].Source < rows[j].Source
	})

	typeWidth := len("TYPE")
	langWidth := len("LANG")
	slugWidth := len("SLUG")
	draftWidth := len("DRAFT")

	for _, row := range rows {
		if len(row.Type) > typeWidth {
			typeWidth = len(row.Type)
		}
		if len(row.Lang) > langWidth {
			langWidth = len(row.Lang)
		}
		if len(row.Slug) > slugWidth {
			slugWidth = len(row.Slug)
		}
	}

	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
		typeWidth, "TYPE",
		langWidth, "LANG",
		slugWidth, "SLUG",
		draftWidth, "DRAFT",
		"TITLE",
	)

	for _, row := range rows {
		draft := "false"
		if row.Draft {
			draft = "true"
		}
		fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
			typeWidth, row.Type,
			langWidth, row.Lang,
			slugWidth, row.Slug,
			draftWidth, draft,
			row.Title,
		)
	}

	fmt.Println("")
	fmt.Printf("%d document(s)\n", len(rows))
	return nil
}

func runGraph(cfg *config.Config) error {
	graph, err := loadGraph(cfg, true)
	if err != nil {
		return err
	}

	fmt.Println("Site graph")
	fmt.Println("----------")
	fmt.Printf("documents: %d\n", len(graph.Documents))
	fmt.Printf("urls: %d\n", len(graph.ByURL))
	fmt.Printf("languages: %d\n", len(graph.ByLang))
	fmt.Printf("types: %d\n", len(graph.ByType))
	fmt.Println("")

	fmt.Println("By language:")
	langs := sortedKeysDocs(graph.ByLang)
	for _, lang := range langs {
		fmt.Printf("- %s: %d\n", lang, len(graph.ByLang[lang]))
	}
	fmt.Println("")

	fmt.Println("By type:")
	types := sortedKeysDocs(graph.ByType)
	for _, typ := range types {
		fmt.Printf("- %s: %d\n", typ, len(graph.ByType[typ]))
	}
	fmt.Println("")

	if graph.Taxonomies.Values != nil && len(graph.Taxonomies.Values) > 0 {
		fmt.Println("Taxonomies:")
		for _, name := range graph.Taxonomies.OrderedNames() {
			def := graph.Taxonomies.Definition(name)
			terms := graph.Taxonomies.Values[name]
			fmt.Printf("- %s (%s): %d term(s)\n", name, def.DisplayTitle(cfg.DefaultLang), len(terms))

			for _, term := range graph.Taxonomies.OrderedTerms(name) {
				fmt.Printf("  - %s: %d document(s)\n", term, len(terms[term]))
			}
		}
		fmt.Println("")
	}

	fmt.Println("Documents:")
	rows := make([]contentRow, 0, len(graph.Documents))
	for _, doc := range graph.Documents {
		rows = append(rows, contentRow{
			Type:   doc.Type,
			Lang:   doc.Lang,
			Title:  doc.Title,
			Slug:   doc.Slug,
			URL:    doc.URL,
			Draft:  doc.Draft,
			Source: doc.SourcePath,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].URL != rows[j].URL {
			return rows[i].URL < rows[j].URL
		}
		return rows[i].Source < rows[j].Source
	})

	for _, row := range rows {
		fmt.Printf("- %s [%s/%s] %s\n", row.URL, row.Type, row.Lang, row.Source)
	}

	return nil
}

func runExport(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry content export <bundle.zip>")
	}
	target := strings.TrimSpace(args[3])
	if target == "" {
		return fmt.Errorf("bundle path must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	for _, rel := range []string{cfg.ContentDir, cfg.DataDir} {
		if err := addPathToZip(zw, rel, rel); err != nil {
			return err
		}
	}
	if _, err := os.Stat(consts.ConfigFilePath); err == nil {
		if err := addPathToZip(zw, consts.ConfigFilePath, consts.ConfigFilePath); err != nil {
			return err
		}
	}

	fmt.Printf("exported content bundle to %s\n", target)
	return nil
}

func runImport(cfg *config.Config, args []string) error {
	if len(args) < 5 {
		return fmt.Errorf("usage: foundry content import [markdown|wordpress] <source>")
	}
	switch strings.TrimSpace(args[3]) {
	case "markdown":
		return importMarkdownTree(cfg, strings.TrimSpace(args[4]))
	case "wordpress":
		return importWordPress(cfg, strings.TrimSpace(args[4]))
	default:
		return fmt.Errorf("unknown import type: %s", args[3])
	}
}

func runMigrate(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry content migrate [layout|field-rename] ...")
	}
	switch strings.TrimSpace(args[3]) {
	case "layout":
		if len(args) < 6 {
			return fmt.Errorf("usage: foundry content migrate layout <from> <to> [--dry-run]")
		}
		return migrateLayouts(cfg, strings.TrimSpace(args[4]), strings.TrimSpace(args[5]), hasDryRunFlag(args[6:]))
	case "field-rename":
		if len(args) < 7 {
			return fmt.Errorf("usage: foundry content migrate field-rename <schema> <old> <new> [--dry-run]")
		}
		return migrateFieldRename(cfg, strings.TrimSpace(args[4]), strings.TrimSpace(args[5]), strings.TrimSpace(args[6]), hasDryRunFlag(args[7:]))
	default:
		return fmt.Errorf("unknown migrate target: %s", args[3])
	}
}

func hasDryRunFlag(args []string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == "--dry-run" {
			return true
		}
	}
	return false
}

func loadGraph(cfg *config.Config, includeDrafts bool) (*content.SiteGraph, error) {
	graph, _, err := site.LoadConfiguredGraph(context.Background(), cfg, includeDrafts)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

func addPathToZip(zw *zip.Writer, root, source string) error {
	return filepath.Walk(source, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Dir(root), current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		writer, err := zw.Create(rel)
		if err != nil {
			return err
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}

func importMarkdownTree(cfg *config.Config, source string) error {
	return filepath.Walk(source, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(current)) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		targetRoot := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir)
		if strings.Contains(strings.ToLower(filepath.ToSlash(rel)), "/posts/") || strings.HasPrefix(strings.ToLower(filepath.ToSlash(rel)), "posts/") {
			targetRoot = filepath.Join(cfg.ContentDir, cfg.Content.PostsDir)
			rel = path.Base(filepath.ToSlash(rel))
		}
		target := filepath.Join(targetRoot, filepath.Base(rel))
		return copyFile(current, target)
	})
}

var (
	wpItemRE      = regexp.MustCompile(`(?s)<item>(.*?)</item>`)
	wpTitleRE     = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	wpSlugRE      = regexp.MustCompile(`(?s)<wp:post_name>(.*?)</wp:post_name>`)
	wpTypeRE      = regexp.MustCompile(`(?s)<wp:post_type>(.*?)</wp:post_type>`)
	wpStatusRE    = regexp.MustCompile(`(?s)<wp:status>(.*?)</wp:status>`)
	wpContentRE   = regexp.MustCompile(`(?s)<content:encoded><!\\[CDATA\\[(.*?)\\]\\]></content:encoded>`)
	wpDateRE      = regexp.MustCompile(`(?s)<wp:post_date>(.*?)</wp:post_date>`)
	xmlTagStripRE = regexp.MustCompile(`<[^>]+>`)
)

func importWordPress(cfg *config.Config, source string) error {
	body, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	items := wpItemRE.FindAllStringSubmatch(string(body), -1)
	for _, item := range items {
		if len(item) != 2 {
			continue
		}
		kind := strings.TrimSpace(extractMatch(wpTypeRE, item[1]))
		if kind != "post" && kind != "page" {
			continue
		}
		slug := normalizeSlug(strings.TrimSpace(extractMatch(wpSlugRE, item[1])))
		if slug == "" {
			slug = normalizeSlug(strings.TrimSpace(extractMatch(wpTitleRE, item[1])))
		}
		if slug == "" {
			continue
		}
		title := strings.TrimSpace(htmlUnescape(extractMatch(wpTitleRE, item[1])))
		status := strings.TrimSpace(extractMatch(wpStatusRE, item[1]))
		contentBody := htmlUnescape(extractMatch(wpContentRE, item[1]))
		date := strings.TrimSpace(extractMatch(wpDateRE, item[1]))

		targetDir := filepath.Join(cfg.ContentDir, cfg.Content.PagesDir)
		layout := cfg.Content.DefaultLayoutPage
		if kind == "post" {
			targetDir = filepath.Join(cfg.ContentDir, cfg.Content.PostsDir)
			layout = cfg.Content.DefaultLayoutPost
		}
		draft := "false"
		if status != "publish" {
			draft = "true"
		}
		frontmatter := fmt.Sprintf("---\ntitle: %s\nslug: %s\nlayout: %s\ndraft: %s\n", yamlScalar(title), slug, layout, draft)
		if date != "" && kind == "post" {
			frontmatter += fmt.Sprintf("date: %s\n", strings.Split(date, " ")[0])
		}
		frontmatter += "---\n\n" + strings.TrimSpace(xmlTagStripRE.ReplaceAllString(contentBody, "")) + "\n"
		if err := writeImportedContentFile(filepath.Join(targetDir, slug+".md"), frontmatter); err != nil {
			return err
		}
	}
	return nil
}

func migrateLayouts(cfg *config.Config, from, to string, dryRun bool) error {
	return rewriteMarkdownFiles(cfg, func(path string, fm *content.FrontMatter, body string) (string, bool, error) {
		if fm == nil || strings.TrimSpace(fm.Layout) != from {
			return "", false, nil
		}
		fm.Layout = to
		return marshalFrontMatter(fm, body)
	}, dryRun)
}

func migrateFieldRename(cfg *config.Config, schema, from, to string, dryRun bool) error {
	return rewriteMarkdownFiles(cfg, func(path string, fm *content.FrontMatter, body string) (string, bool, error) {
		if fm == nil || fm.Fields == nil || strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
			return "", false, nil
		}
		if schema != "" {
			if value, ok := fm.Params["schema"].(string); !ok || strings.TrimSpace(value) != schema {
				return "", false, nil
			}
		}
		value, ok := fm.Fields[from]
		if !ok {
			return "", false, nil
		}
		delete(fm.Fields, from)
		fm.Fields[to] = value
		return marshalFrontMatter(fm, body)
	}, dryRun)
}

func rewriteMarkdownFiles(cfg *config.Config, rewrite func(path string, fm *content.FrontMatter, body string) (string, bool, error), dryRun bool) error {
	for _, root := range []string{
		filepath.Join(cfg.ContentDir, cfg.Content.PagesDir),
		filepath.Join(cfg.ContentDir, cfg.Content.PostsDir),
	} {
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info == nil || info.IsDir() || strings.ToLower(filepath.Ext(path)) != ".md" {
				return err
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fm, body, err := content.ParseDocument(src)
			if err != nil {
				return err
			}
			updated, changed, err := rewrite(path, fm, body)
			if err != nil || !changed {
				return err
			}
			if dryRun {
				fmt.Printf("would update %s\n", path)
				return nil
			}
			return os.WriteFile(path, []byte(updated), 0o644)
		}); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func marshalFrontMatter(fm *content.FrontMatter, body string) (string, bool, error) {
	out, err := yaml.Marshal(fm)
	if err != nil {
		return "", false, err
	}
	return "---\n" + string(out) + "---\n\n" + strings.TrimLeft(body, "\n"), true, nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeImportedContentFile(path, body string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return writeNewContentFile(path, body)
}

func extractMatch(re *regexp.Regexp, body string) string {
	match := re.FindStringSubmatch(body)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func htmlUnescape(value string) string {
	replacer := strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&", "&quot;", `"`, "&#39;", "'")
	return replacer.Replace(strings.TrimSpace(value))
}

func yamlScalar(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	out, err := yaml.Marshal(value)
	if err != nil {
		return `""`
	}
	return strings.TrimSpace(string(out))
}

func normalizeSlug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

func writeNewContentFile(path, body string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path must not be empty")
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}

	fmt.Printf("created %s\n", path)
	return nil
}

func sortedKeysDocs[T any](m map[string][]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func init() {
	registry.Register(command{})
}
