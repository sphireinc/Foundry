package fields

func Normalize(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}
