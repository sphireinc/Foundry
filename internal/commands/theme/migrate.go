package themecmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/consts"
	"github.com/sphireinc/foundry/internal/theme"
	"gopkg.in/yaml.v3"
)

func runMigrateFieldContracts(cfg *config.Config, args []string) error {
	if len(args) < 4 || strings.TrimSpace(args[3]) != "field-contracts" {
		return fmt.Errorf("usage: foundry theme migrate field-contracts")
	}

	body, err := os.ReadFile(consts.ConfigFilePath)
	if err != nil {
		return err
	}
	var siteCfg config.Config
	if err := config.UnmarshalYAML(body, &siteCfg); err != nil {
		return err
	}
	if len(siteCfg.Fields.Schemas) == 0 {
		cliout.Println(cliout.Warning("no legacy config-owned field schemas found in content/config/site.yaml"))
		return nil
	}

	manifestPath := filepath.Join(cfg.ThemesDir, cfg.Theme, "theme.yaml")
	manifest, err := theme.LoadManifest(cfg.ThemesDir, cfg.Theme)
	if err != nil {
		return err
	}
	if len(manifest.FieldContracts) > 0 {
		return fmt.Errorf("theme %q already defines field_contracts; migrate manually or clear them first", cfg.Theme)
	}

	migrated := make([]theme.FieldContract, 0, len(siteCfg.Fields.Schemas))
	for key, set := range siteCfg.Fields.Schemas {
		contractKey := strings.TrimSpace(strings.ToLower(key))
		if contractKey == "" {
			continue
		}
		target := theme.FieldContractTarget{Scope: "document"}
		switch contractKey {
		case "default":
			target.Types = []string{"page", "post"}
		default:
			target.Types = []string{contractKey}
		}
		migrated = append(migrated, theme.FieldContract{
			Key:         "migrated-" + strings.ReplaceAll(contractKey, "_", "-"),
			Title:       "Migrated " + strings.ToUpper(contractKey[:1]) + contractKey[1:] + " Fields",
			Description: "Migrated from legacy content/config/site.yaml fields schema.",
			Target:      target,
			Fields:      append([]config.FieldDefinition(nil), set.Fields...),
		})
	}
	if len(migrated) == 0 {
		cliout.Println(cliout.Warning("no migratable field schemas found"))
		return nil
	}

	manifest.FieldContracts = migrated
	rendered, err := yaml.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath, rendered, 0o644); err != nil {
		return err
	}
	if err := config.RemoveTopLevelKey(consts.ConfigFilePath, "fields"); err != nil {
		return err
	}

	cliout.Successf("Migrated %d field contract(s) into theme %q", len(migrated), cfg.Theme)
	fmt.Printf("%s %s\n", cliout.Label("Theme Manifest:"), manifestPath)
	fmt.Printf("%s %s\n", cliout.Label("Config Updated:"), consts.ConfigFilePath)
	fmt.Println("")
	cliout.Println(cliout.Heading("Next steps:"))
	fmt.Println("1. Review theme.yaml field_contracts")
	fmt.Println("2. Move shared values into content/custom-fields.yaml if needed")
	fmt.Println("3. Open the admin Custom Fields and Editor screens to verify the theme contract")
	return nil
}
