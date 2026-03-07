package router

import (
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/content"
)

type Resolver struct {
	cfg *config.Config
}

func NewResolver(cfg *config.Config) *Resolver {
	return &Resolver{cfg: cfg}
}

func (r *Resolver) AssignURLs(graph *content.SiteGraph) error {
	graph.ByURL = make(map[string]*content.Document)

	for _, doc := range graph.Documents {
		key := doc.Type + "_default"
		if !doc.IsDefault {
			key = doc.Type + "_i18n"
		}

		pattern, ok := r.cfg.Permalinks[key]
		if !ok {
			return fmt.Errorf("missing permalink pattern for %q", key)
		}

		url := pattern
		url = strings.ReplaceAll(url, ":lang", doc.Lang)
		url = strings.ReplaceAll(url, ":slug", doc.Slug)

		if doc.Type == "page" && doc.Slug == r.cfg.Content.DefaultPageSlugIndex {
			if doc.IsDefault {
				url = "/"
			} else {
				url = "/" + doc.Lang + "/"
			}
		}

		if !strings.HasPrefix(url, "/") {
			url = "/" + url
		}
		if !strings.HasSuffix(url, "/") {
			url += "/"
		}

		doc.URL = url
		graph.ByURL[doc.URL] = doc
	}

	return nil
}

func (r *Resolver) TaxonomyURL(lang, taxonomyName, term string) string {
	var url string
	if lang == "" || lang == r.cfg.DefaultLang {
		url = fmt.Sprintf("/%s/%s/", taxonomyName, term)
	} else {
		url = fmt.Sprintf("/%s/%s/%s/", lang, taxonomyName, term)
	}

	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}

	return url
}
