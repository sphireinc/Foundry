package plugin

import (
	"fmt"
	"github.com/sphireinc/foundry/internal/cliout"
	"github.com/sphireinc/foundry/internal/config"
	"github.com/sphireinc/foundry/internal/plugins"
	"strings"
)

func runInit(cfg *config.Config, args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: foundry plugin init <name> [--runtime rpc|compiled]")
	}
	runtime := "rpc"
	rest := args[4:]
	if len(rest) == 2 && rest[0] == "--runtime" {
		runtime = rest[1]
	} else if len(rest) == 1 && strings.HasPrefix(rest[0], "--runtime=") {
		runtime = strings.TrimPrefix(rest[0], "--runtime=")
	} else if len(rest) != 0 {
		return fmt.Errorf("usage: foundry plugin init <name> [--runtime rpc|compiled]")
	}
	path, err := plugins.Scaffold(cfg.PluginsDir, args[3], runtime)
	if err != nil {
		return err
	}
	cliout.Successf("Created %s plugin %q at %s (not enabled)", runtime, args[3], path)
	fmt.Printf("Read %s/README.md for test/build instructions.\nNext: foundry plugin validate %q --security\n", path, args[3])
	return nil
}

func printAuthorValidationHints(project plugins.Project, name string, validationErr error) {
	if validationErr == nil {
		return
	}
	message := validationErr.Error()
	switch {
	case strings.Contains(message, "security validation failed"):
		if meta, err := project.Metadata(name); err == nil {
			printSecurityValidation(name, plugins.AnalyzeInstalled(meta))
		}
		fmt.Printf("  Hint: Inspect foundry plugin security %q. Declare only the permissions the implementation needs in plugin.yaml, or remove the detected capability.\n", name)
	case strings.Contains(message, "no .go files"):
		fmt.Println("  Hint: Add the compiled plugin's Go source and registration, or declare runtime.mode: rpc with a built runtime.command. Run go test and go build separately.")
	case strings.Contains(message, "does not exist"):
		fmt.Printf("  Hint: Initialize a new local plugin with foundry plugin init %q, or install the missing dependency before enabling it.\n", name)
	default:
		fmt.Println("  Hint: Check the plugin.yaml field/path named in the error. Compare its API, version, dependencies, and runtime settings with docs/extension-authoring.md.")
	}
}
