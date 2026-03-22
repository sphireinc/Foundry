package fields

import (
	"fmt"
	"strings"

	"github.com/sphireinc/foundry/internal/config"
)

type Definition = config.FieldDefinition
type SchemaSet = config.FieldSchemaSet

func Normalize(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func DefinitionsFor(cfg *config.Config, kind string) []Definition {
	if cfg == nil {
		return nil
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if schema, ok := cfg.Fields.Schemas[kind]; ok {
		return append([]Definition(nil), schema.Fields...)
	}
	if schema, ok := cfg.Fields.Schemas["default"]; ok {
		return append([]Definition(nil), schema.Fields...)
	}
	return nil
}

func ApplyDefaults(values map[string]any, defs []Definition) map[string]any {
	out := cloneMap(values)
	for _, def := range defs {
		if _, ok := out[def.Name]; ok {
			continue
		}
		if def.Default != nil {
			out[def.Name] = cloneValue(def.Default)
			continue
		}
		switch normalizeType(def.Type) {
		case "object":
			out[def.Name] = ApplyDefaults(nil, def.Fields)
		case "repeater":
			out[def.Name] = []any{}
		}
	}
	return out
}

func Validate(values map[string]any, defs []Definition, allowAnything bool) []error {
	var errs []error
	values = Normalize(values)
	known := make(map[string]Definition, len(defs))
	for _, def := range defs {
		known[def.Name] = def
		value, ok := values[def.Name]
		if !ok {
			if def.Required {
				errs = append(errs, fmt.Errorf("field %q is required", def.Name))
			}
			continue
		}
		errs = append(errs, validateValue(def.Name, value, def)...)
	}
	if !allowAnything {
		for name := range values {
			if _, ok := known[name]; !ok {
				errs = append(errs, fmt.Errorf("field %q is not allowed by schema", name))
			}
		}
	}
	return errs
}

func validateValue(path string, value any, def Definition) []error {
	var errs []error
	switch normalizeType(def.Type) {
	case "text", "textarea":
		if _, ok := value.(string); !ok {
			errs = append(errs, fmt.Errorf("field %q must be a string", path))
		}
	case "bool":
		if _, ok := value.(bool); !ok {
			errs = append(errs, fmt.Errorf("field %q must be a boolean", path))
		}
	case "number":
		switch value.(type) {
		case int, int64, float64, float32:
		default:
			errs = append(errs, fmt.Errorf("field %q must be numeric", path))
		}
	case "select":
		text, ok := value.(string)
		if !ok {
			errs = append(errs, fmt.Errorf("field %q must be a string", path))
		} else if len(def.Enum) > 0 && !contains(def.Enum, text) {
			errs = append(errs, fmt.Errorf("field %q must be one of %s", path, strings.Join(def.Enum, ", ")))
		}
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("field %q must be an object", path))
		} else {
			for _, err := range Validate(obj, def.Fields, false) {
				errs = append(errs, fmt.Errorf("%s.%v", path, err))
			}
		}
	case "repeater":
		items, ok := value.([]any)
		if !ok {
			errs = append(errs, fmt.Errorf("field %q must be a list", path))
		} else if def.Item != nil {
			for index, item := range items {
				errs = append(errs, validateValue(fmt.Sprintf("%s[%d]", path, index), item, *def.Item)...)
			}
		}
	}
	return errs
}

func normalizeType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "string":
		return "text"
	case "textarea":
		return "textarea"
	case "bool", "boolean":
		return "bool"
	case "number", "int", "float":
		return "number"
	case "select", "enum":
		return "select"
	case "object":
		return "object"
	case "repeater", "list", "array":
		return "repeater"
	default:
		return "text"
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i := range typed {
			out[i] = cloneValue(typed[i])
		}
		return out
	default:
		return typed
	}
}
