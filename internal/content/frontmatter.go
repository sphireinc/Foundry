package content

import (
	"bytes"
	"fmt"

	"github.com/adrg/frontmatter"
)

func ParseDocument(src []byte) (*FrontMatter, string, error) {
	var fm FrontMatter

	body, err := frontmatter.Parse(bytes.NewReader(src), &fm)
	if err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	// The YAML parser uses interface-keyed maps for nested inline values.
	// Editorial metadata is also exposed through JSON document APIs.
	if value, ok := fm.Params["editorial"]; ok {
		fm.Params["editorial"] = normalizeEditorialValue(value)
	}

	return &fm, string(body), nil
}

func normalizeEditorialValue(value any) any {
	switch value := value.(type) {
	case map[any]any:
		result := make(map[string]any, len(value))
		for key, item := range value {
			result[fmt.Sprint(key)] = normalizeEditorialValue(item)
		}
		return result
	case map[string]any:
		for key, item := range value {
			value[key] = normalizeEditorialValue(item)
		}
	case []any:
		for i, item := range value {
			value[i] = normalizeEditorialValue(item)
		}
	}
	return value
}
