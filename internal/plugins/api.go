package plugins

import (
	"fmt"
	"strings"
)

const SupportedFoundryAPI = "v1"

func validateMetadataCompatibility(meta Metadata) error {
	if strings.TrimSpace(meta.FoundryAPI) == "" {
		return fmt.Errorf("missing required field %q", "foundry_api")
	}
	if meta.FoundryAPI != SupportedFoundryAPI {
		return fmt.Errorf("unsupported foundry_api %q (supported: %s)", meta.FoundryAPI, SupportedFoundryAPI)
	}

	if strings.TrimSpace(meta.MinFoundryVersion) == "" {
		return fmt.Errorf("missing required field %q", "min_foundry_version")
	}

	return nil
}
