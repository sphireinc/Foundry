package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrResultTooLarge = errors.New("mcp: result exceeds maximum size")

func MarshalBounded(value any, maxBytes int) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode result: %w", err)
	}
	if maxBytes > 0 && len(data) > maxBytes {
		return nil, fmt.Errorf("%w: %d bytes exceeds limit of %d", ErrResultTooLarge, len(data), maxBytes)
	}
	return data, nil
}

type Window struct {
	Start      int
	End        int
	Total      int
	More       bool
	NextCursor string
}

func Paginate(total, limit int, cursor string) (Window, error) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}

	offset, err := parseCursor(cursor)
	if err != nil {
		return Window{}, err
	}
	if offset > total {
		offset = total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	window := Window{
		Start: offset,
		End:   end,
		Total: total,
		More:  end < total,
	}
	if window.More {
		window.NextCursor = strconv.Itoa(end)
	}
	return window, nil
}

func parseCursor(cursor string) (int, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(cursor)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("mcp: invalid cursor %q", cursor)
	}
	return value, nil
}
