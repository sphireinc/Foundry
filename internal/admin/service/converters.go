package service

import (
	"github.com/sphireinc/foundry/internal/admin/types"
	adminui "github.com/sphireinc/foundry/internal/admin/ui"
	"github.com/sphireinc/foundry/internal/plugins"
	"github.com/sphireinc/foundry/internal/theme"
)

func toValidationDiagnostics(in []theme.ValidationDiagnostic) []types.ValidationDiagnostic {
	out := make([]types.ValidationDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, types.ValidationDiagnostic{
			Severity: diagnostic.Severity,
			Path:     diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func toAdminThemeDiagnostics(in []adminui.Diagnostic) []types.ValidationDiagnostic {
	out := make([]types.ValidationDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, types.ValidationDiagnostic{
			Severity: diagnostic.Severity,
			Path:     diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func toPluginDiagnostics(in []plugins.ValidationDiagnostic) []types.ValidationDiagnostic {
	out := make([]types.ValidationDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		out = append(out, types.ValidationDiagnostic{
			Severity: diagnostic.Severity,
			Path:     diagnostic.Path,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func toPluginDependencies(in []plugins.Dependency) []types.PluginDependency {
	out := make([]types.PluginDependency, 0, len(in))
	for _, dep := range in {
		out = append(out, types.PluginDependency{
			Name:     dep.Name,
			Version:  dep.Version,
			Optional: dep.Optional,
		})
	}
	return out
}
