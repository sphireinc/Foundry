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

	return &fm, string(body), nil
}
