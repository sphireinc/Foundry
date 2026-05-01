package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if err := run("go", "run", "./scripts/cmd/e2e-cleanup"); err != nil {
		fatalf("pre-clean failed: %v", err)
	}

	testErr := run("npx", "playwright", "test", "--config=playwright.readme.config.js")
	cleanErr := run("go", "run", "./scripts/cmd/e2e-cleanup")
	if cleanErr != nil {
		fatalf("post-clean failed: %v", cleanErr)
	}
	if testErr != nil {
		if exitErr, ok := testErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatalf("playwright failed: %v", testErr)
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	return cmd.Run()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
