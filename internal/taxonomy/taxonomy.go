package taxonomy

import (
	"sort"
	"strings"
)

type Entry struct {
	DocumentID string
	URL        string
	Lang       string
	Type       string
	Title      string
	Slug       string
}

type Definition struct {
	Name          string
	Title         string
	Labels        map[string]string
	ArchiveLayout string
	TermLayout    string
	Order         string
}

type Index struct {
	Values      map[string]map[string][]Entry
	Definitions map[string]Definition
}

func New(definitions map[string]Definition) *Index {
	idx := &Index{
		Values:      make(map[string]map[string][]Entry),
		Definitions: make(map[string]Definition),
	}

	for name, def := range definitions {
		idx.Definitions[name] = normalizeDefinition(name, def)
	}

	return idx
}

func (i *Index) AddDocument(docID, url, lang, docType, title, slug string, taxonomies map[string][]string) {
	for taxonomyName, terms := range taxonomies {
		i.EnsureDefinition(taxonomyName)

		if _, ok := i.Values[taxonomyName]; !ok {
			i.Values[taxonomyName] = make(map[string][]Entry)
		}

		for _, term := range terms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}

			i.Values[taxonomyName][term] = append(i.Values[taxonomyName][term], Entry{
				DocumentID: docID,
				URL:        url,
				Lang:       lang,
				Type:       docType,
				Title:      title,
				Slug:       slug,
			})
		}
	}
}

func (i *Index) EnsureDefinition(name string) {
	if i.Definitions == nil {
		i.Definitions = make(map[string]Definition)
	}
	if _, ok := i.Definitions[name]; ok {
		return
	}
	i.Definitions[name] = normalizeDefinition(name, Definition{Name: name})
}

func (i *Index) Definition(name string) Definition {
	if i.Definitions == nil {
		return normalizeDefinition(name, Definition{Name: name})
	}
	if def, ok := i.Definitions[name]; ok {
		return normalizeDefinition(name, def)
	}
	return normalizeDefinition(name, Definition{Name: name})
}

func (i *Index) OrderedNames() []string {
	names := make([]string, 0, len(i.Values))
	for name := range i.Values {
		names = append(names, name)
	}

	sort.Slice(names, func(a, b int) bool {
		defA := i.Definition(names[a])
		defB := i.Definition(names[b])

		titleA := strings.ToLower(defA.DisplayTitle(""))
		titleB := strings.ToLower(defB.DisplayTitle(""))

		if titleA != titleB {
			return titleA < titleB
		}
		return names[a] < names[b]
	})

	return names
}

func (i *Index) OrderedTerms(name string) []string {
	termsMap, ok := i.Values[name]
	if !ok {
		return nil
	}

	terms := make([]string, 0, len(termsMap))
	for term := range termsMap {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	return terms
}

func (d Definition) DisplayTitle(lang string) string {
	if lang != "" && d.Labels != nil {
		if label := strings.TrimSpace(d.Labels[lang]); label != "" {
			return label
		}
	}
	if title := strings.TrimSpace(d.Title); title != "" {
		return title
	}
	if name := strings.TrimSpace(d.Name); name != "" {
		return name
	}
	return "taxonomy"
}

func (d Definition) EffectiveTermLayout() string {
	if strings.TrimSpace(d.TermLayout) != "" {
		return d.TermLayout
	}
	if strings.TrimSpace(d.ArchiveLayout) != "" {
		return d.ArchiveLayout
	}
	return "list"
}

func normalizeDefinition(name string, d Definition) Definition {
	d.Name = strings.TrimSpace(name)
	if strings.TrimSpace(d.Title) == "" {
		d.Title = d.Name
	}
	if d.Labels == nil {
		d.Labels = map[string]string{}
	}
	return d
}
