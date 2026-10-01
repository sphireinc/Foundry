package media

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"golang.org/x/net/html"
)

type AccessibilityIssue struct {
	Source  string `json:"source"`
	Image   string `json:"image"`
	Message string `json:"message"`
}

// EnrichHTML preserves existing srcset, sizes, dimensions and meaningful inline alt.
// Only local media references receive metadata and generated variants.
func EnrichHTML(input []byte, cfg *config.Config, index ImageIndex) ([]byte, error) {
	tokenizer := html.NewTokenizer(bytes.NewReader(input))
	var output bytes.Buffer
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if errors.Is(tokenizer.Err(), io.EOF) {
				return output.Bytes(), nil
			}
			return nil, tokenizer.Err()
		}
		raw := append([]byte(nil), tokenizer.Raw()...)
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			output.Write(raw)
			continue
		}
		token := tokenizer.Token()
		if token.Data != "img" {
			output.Write(raw)
			continue
		}
		publicURL := ImagePublicURL(attribute(token, "src"))
		reference := ""
		for _, collection := range SupportedCollections {
			if strings.HasPrefix(publicURL, "/"+collection+"/") {
				reference = ReferenceScheme + strings.TrimPrefix(publicURL, "/")
				break
			}
		}
		if reference == "" {
			output.Write(raw)
			continue
		}
		ref, err := ResolveReference(reference)
		if err != nil {
			return nil, err
		}
		root, err := CollectionRoot(cfg, ref.Collection)
		if err != nil {
			return nil, err
		}
		filename := filepath.Join(root, filepath.FromSlash(ref.Path))
		metadata, err := ReadImageMetadata(root, filename)
		if err != nil {
			return nil, err
		}
		changed := false
		if strings.TrimSpace(attribute(token, "alt")) == "" {
			if metadata.Decorative {
				setAttribute(&token, "alt", "")
				setAttribute(&token, "role", "presentation")
				changed = true
			} else if metadata.Alt != "" {
				setAttribute(&token, "alt", metadata.Alt)
				changed = true
			}
		}
		if entry, ok := index[publicURL]; ok {
			if !hasAttribute(token, "srcset") && len(entry.Variants) > 0 {
				candidates := make([]string, 0, len(entry.Variants))
				for _, variant := range entry.Variants {
					candidates = append(candidates, variant.URL+" "+strconv.Itoa(variant.Width)+"w")
				}
				setAttribute(&token, "srcset", strings.Join(candidates, ", "))
				if !hasAttribute(token, "sizes") {
					setAttribute(&token, "sizes", "100vw")
				}
				changed = true
			}
			if !hasAttribute(token, "width") && !hasAttribute(token, "height") {
				setAttribute(&token, "width", strconv.Itoa(entry.Width))
				setAttribute(&token, "height", strconv.Itoa(entry.Height))
				changed = true
			}
		}
		if changed {
			output.WriteString(token.String())
		} else {
			output.Write(raw)
		}
	}
}

func CheckHTML(input []byte, source string) []AccessibilityIssue {
	tokenizer := html.NewTokenizer(bytes.NewReader(input))
	var issues []AccessibilityIssue
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			return issues
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data != "img" {
			continue
		}
		src := attribute(token, "src")
		if ref, err := ResolveReference(src); err == nil && ref.Kind != KindImage {
			continue
		}
		if strings.TrimSpace(attribute(token, "alt")) != "" {
			continue
		}
		if hasAttribute(token, "alt") && (attribute(token, "role") == "presentation" || attribute(token, "role") == "none" || attribute(token, "aria-hidden") == "true") {
			continue
		}
		issues = append(issues, AccessibilityIssue{Source: source, Image: src, Message: "image requires descriptive alt text or explicit decorative metadata/role"})
	}
}

func RequireAccessibleHTML(input []byte, source string) error {
	issues := CheckHTML(input, source)
	if len(issues) == 0 {
		return nil
	}
	return fmt.Errorf("%s: %s (%s)", source, issues[0].Message, issues[0].Image)
}
func attribute(token html.Token, name string) string {
	for _, attr := range token.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}
func hasAttribute(token html.Token, name string) bool {
	for _, attr := range token.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
}
func setAttribute(token *html.Token, name, value string) {
	for i := range token.Attr {
		if token.Attr[i].Key == name {
			token.Attr[i].Val = value
			return
		}
	}
	token.Attr = append(token.Attr, html.Attribute{Key: name, Val: value})
}
