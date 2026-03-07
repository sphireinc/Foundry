package taxonomy

type Entry struct {
	DocumentID string
	URL        string
	Lang       string
	Type       string
	Title      string
	Slug       string
}

type Index struct {
	Values map[string]map[string][]Entry
}

func New() *Index {
	return &Index{
		Values: make(map[string]map[string][]Entry),
	}
}

func (i *Index) AddDocument(docID, url, lang, docType, title, slug string, taxonomies map[string][]string) {
	for taxonomy, terms := range taxonomies {
		if _, ok := i.Values[taxonomy]; !ok {
			i.Values[taxonomy] = make(map[string][]Entry)
		}

		for _, term := range terms {
			i.Values[taxonomy][term] = append(i.Values[taxonomy][term], Entry{
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
