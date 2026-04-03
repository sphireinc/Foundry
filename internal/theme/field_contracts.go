package theme

import (
	"strings"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
)

func DocumentFieldDefinitions(themesDir, themeName, docType, layout, slug string) []foundryconfig.FieldDefinition {
	manifest, err := LoadManifest(themesDir, themeName)
	if err != nil || manifest == nil {
		return nil
	}

	docType = strings.ToLower(strings.TrimSpace(docType))
	layout = strings.ToLower(strings.TrimSpace(layout))
	slug = strings.ToLower(strings.TrimSpace(slug))

	var defs []foundryconfig.FieldDefinition
	for _, contract := range manifest.FieldContracts {
		if !matchesDocumentContract(contract.Target, docType, layout, slug) {
			continue
		}
		defs = append(defs, cloneFieldDefinitions(contract.Fields)...)
	}
	return defs
}

func matchesDocumentContract(target FieldContractTarget, docType, layout, slug string) bool {
	if strings.ToLower(strings.TrimSpace(target.Scope)) != "document" {
		return false
	}
	if !matchesOneOf(target.Types, docType) {
		return false
	}
	if !matchesOneOf(target.Layouts, layout) {
		return false
	}
	if !matchesOneOf(target.Slugs, slug) {
		return false
	}
	return true
}

func matchesOneOf(values []string, target string) bool {
	if len(values) == 0 {
		return true
	}
	target = strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}

func cloneFieldDefinitions(defs []foundryconfig.FieldDefinition) []foundryconfig.FieldDefinition {
	if len(defs) == 0 {
		return nil
	}
	out := make([]foundryconfig.FieldDefinition, 0, len(defs))
	for _, def := range defs {
		entry := def
		entry.Enum = append([]string(nil), def.Enum...)
		entry.Fields = cloneFieldDefinitions(def.Fields)
		if def.Item != nil {
			item := *def.Item
			item.Enum = append([]string(nil), def.Item.Enum...)
			item.Fields = cloneFieldDefinitions(def.Item.Fields)
			if def.Item.Item != nil {
				item.Item = cloneFieldDefinitionPtr(def.Item.Item)
			}
			entry.Item = &item
		}
		out = append(out, entry)
	}
	return out
}

func cloneFieldDefinitionPtr(def *foundryconfig.FieldDefinition) *foundryconfig.FieldDefinition {
	if def == nil {
		return nil
	}
	out := *def
	out.Enum = append([]string(nil), def.Enum...)
	out.Fields = cloneFieldDefinitions(def.Fields)
	if def.Item != nil {
		out.Item = cloneFieldDefinitionPtr(def.Item)
	}
	return &out
}
