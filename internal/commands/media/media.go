package mediacmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/sphireinc/foundry/internal/admin/service"
	"github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/assets"
	"github.com/sphireinc/foundry/internal/commands/registry"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/media"
)

type command struct{}

func (command) Name() string         { return "media" }
func (command) Summary() string      { return "Audit, optimize, and recover media" }
func (command) Group() string        { return "asset commands" }
func (command) RequiresConfig() bool { return true }
func (command) Details() []string {
	return []string{"foundry media audit [--json]", "foundry media optimize", "foundry media trash <reference> [--apply]", "foundry media restore <trash-path>"}
}
func (command) Run(cfg *config.Config, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: foundry media [audit|optimize|trash|restore]")
	}
	switch args[2] {
	case "audit":
		if len(args) != 3 && (len(args) != 4 || args[3] != "--json") {
			return fmt.Errorf("usage: foundry media audit [--json]")
		}
		report, err := media.Audit(cfg)
		if err != nil {
			return err
		}
		if len(args) == 4 {
			return printJSON(report)
		}
		fmt.Printf("Media audit: %d file(s), %d accessibility issue(s), %d potential orphan(s)\n", len(report.Assets), len(report.Accessibility), len(report.PotentialOrphans))
		for _, issue := range report.Accessibility {
			fmt.Printf("[FAIL] %s: %s (%s)\n", issue.Source, issue.Message, issue.Image)
		}
		for _, reference := range report.PotentialOrphans {
			fmt.Printf("[REVIEW] %s\n", reference)
		}
		for _, note := range report.Notes {
			fmt.Println(note)
		}
		return nil
	case "optimize":
		if len(args) != 3 {
			return fmt.Errorf("usage: foundry media optimize")
		}
		if !cfg.Media.ResponsiveImages {
			return fmt.Errorf("enable media.responsive_images before optimizing")
		}
		if err := assets.Sync(cfg, nil); err != nil {
			return err
		}
		fmt.Println("responsive images built; originals preserved")
		return nil
	case "trash":
		if len(args) < 4 || len(args) > 5 {
			return fmt.Errorf("usage: foundry media trash <reference> [--apply]")
		}
		if len(args) == 5 && args[4] != "--apply" {
			return fmt.Errorf("usage: foundry media trash <reference> [--apply]")
		}
		reference := args[3]
		report, err := media.Audit(cfg)
		if err != nil {
			return err
		}
		if !slices.Contains(report.PotentialOrphans, reference) {
			return fmt.Errorf("media is referenced or absent from the orphan preview: %s", reference)
		}
		if len(args) == 4 {
			fmt.Printf("Preview: would move %s and its metadata to recoverable Trash.\nNo files changed. Run foundry media trash %q --apply after reviewing dynamic/external usage.\n", reference, reference)
			return nil
		}
		svc := service.New(cfg)
		if err := svc.DeleteMedia(context.Background(), reference); err != nil {
			return err
		}
		trash, err := svc.ListMediaTrash(context.Background())
		if err != nil {
			return err
		}
		fmt.Printf("Moved media to Trash: %s\n", reference)
		for _, entry := range trash {
			if entry.CurrentReference == reference && !entry.MetadataOnly {
				fmt.Printf("Recover with: foundry media restore %q\n", entry.Path)
			}
		}
		fmt.Println("Next steps:\n1. Run foundry build to refresh public output.")
		return nil
	case "restore":
		if len(args) != 4 {
			return fmt.Errorf("usage: foundry media restore <trash-path>")
		}
		response, err := service.New(cfg).RestoreMedia(context.Background(), types.MediaLifecycleRequest{Path: args[3]})
		if err != nil {
			return err
		}
		fmt.Printf("Restored media: %s\nNext steps:\n1. Run foundry build to refresh public output.\n", response.RestoredPath)
		return nil
	}
	return fmt.Errorf("unknown media subcommand: %s", args[2])
}
func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
func init() { registry.Register(command{}) }
