package theme

import (
	"strings"

	foundryconfig "github.com/sphireinc/foundry/internal/config"
)

func normalizeFieldContractScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "document", "page":
		return "document"
	case "shared", "global":
		return "shared"
	default:
		return strings.ToLower(strings.TrimSpace(scope))
	}
}

func cloneFieldDefinitions(in []foundryconfig.FieldDefinition) []foundryconfig.FieldDefinition {
	if len(in) == 0 {
		return nil
	}
	out := make([]foundryconfig.FieldDefinition, 0, len(in))
	for _, def := range in {
		cloned := def
		cloned.Enum = append([]string(nil), def.Enum...)
		cloned.Fields = cloneFieldDefinitions(def.Fields)
		if def.Item != nil {
			item := *def.Item
			item.Enum = append([]string(nil), def.Item.Enum...)
			item.Fields = cloneFieldDefinitions(def.Item.Fields)
			cloned.Item = &item
		}
		out = append(out, cloned)
	}
	return out
}

func mergeFieldDefinitions(sets ...[]foundryconfig.FieldDefinition) []foundryconfig.FieldDefinition {
	order := make([]string, 0)
	merged := make(map[string]foundryconfig.FieldDefinition)
	for _, set := range sets {
		for _, def := range set {
			name := strings.TrimSpace(def.Name)
			if name == "" {
				continue
			}
			if _, ok := merged[name]; !ok {
				order = append(order, name)
			}
			merged[name] = def
		}
	}
	out := make([]foundryconfig.FieldDefinition, 0, len(order))
	for _, name := range order {
		out = append(out, merged[name])
	}
	return out
}

func contractMatchesDocument(contract FieldContract, docType, layout, slug string) bool {
	scope := normalizeFieldContractScope(contract.Target.Scope)
	if scope != "document" {
		return false
	}
	if len(contract.Target.Types) > 0 && !containsFold(contract.Target.Types, docType) {
		return false
	}
	if len(contract.Target.Layouts) > 0 && !containsFold(contract.Target.Layouts, layout) {
		return false
	}
	if len(contract.Target.Slugs) > 0 && !containsFold(contract.Target.Slugs, slug) {
		return false
	}
	return true
}

func containsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

func ApplicableDocumentFieldContracts(manifest *Manifest, docType, layout, slug string) []FieldContract {
	if manifest == nil {
		return nil
	}
	out := make([]FieldContract, 0)
	for _, contract := range manifest.FieldContracts {
		if contractMatchesDocument(contract, docType, layout, slug) {
			out = append(out, contract)
		}
	}
	return out
}

func ApplicableDocumentFieldDefinitions(manifest *Manifest, docType, layout, slug string) []foundryconfig.FieldDefinition {
	contracts := ApplicableDocumentFieldContracts(manifest, docType, layout, slug)
	sets := make([][]foundryconfig.FieldDefinition, 0, len(contracts))
	for _, contract := range contracts {
		sets = append(sets, cloneFieldDefinitions(contract.Fields))
	}
	return mergeFieldDefinitions(sets...)
}

func SharedFieldContracts(manifest *Manifest) []FieldContract {
	if manifest == nil {
		return nil
	}
	out := make([]FieldContract, 0)
	for _, contract := range manifest.FieldContracts {
		if normalizeFieldContractScope(contract.Target.Scope) == "shared" {
			out = append(out, contract)
		}
	}
	return out
}
