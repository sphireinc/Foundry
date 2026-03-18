package registry

import (
	"errors"
	"strings"
	"testing"

	"github.com/sphireinc/foundry/internal/config"
)

type testCommand struct {
	name string
	err  error
}

func (c testCommand) Name() string                       { return c.name }
func (c testCommand) Summary() string                    { return "summary" }
func (c testCommand) Group() string                      { return "group" }
func (c testCommand) Details() []string                  { return []string{"detail"} }
func (c testCommand) RequiresConfig() bool               { return true }
func (c testCommand) Run(*config.Config, []string) error { return c.err }

func TestRegistryRegisterLookupRunListUsage(t *testing.T) {
	old := commands
	commands = map[string]Command{}
	defer func() { commands = old }()

	Register(testCommand{name: "alpha"})
	Register(testCommand{name: "beta", err: errors.New("boom")})

	if _, ok := Lookup([]string{"foundry"}); ok {
		t.Fatal("expected lookup to fail for short args")
	}
	if cmd, ok := Lookup([]string{"foundry", "alpha"}); !ok || cmd.Name() != "alpha" {
		t.Fatalf("unexpected lookup result: %#v %v", cmd, ok)
	}

	handled, err := Run(nil, []string{"foundry", "alpha"})
	if !handled || err != nil {
		t.Fatalf("unexpected run result: handled=%v err=%v", handled, err)
	}
	handled, err = Run(nil, []string{"foundry", "beta"})
	if !handled || err == nil {
		t.Fatalf("expected command error, got handled=%v err=%v", handled, err)
	}
	handled, err = Run(nil, []string{"foundry", "missing"})
	if handled || err != nil {
		t.Fatalf("expected unknown command to be ignored, got handled=%v err=%v", handled, err)
	}

	list := List()
	if len(list) != 2 || list[0].Name != "alpha" {
		t.Fatalf("unexpected list: %#v", list)
	}
	usage := Usage()
	if !strings.Contains(usage, "alpha") || !strings.Contains(usage, "detail") {
		t.Fatalf("unexpected usage output: %q", usage)
	}
}

func TestRegistryRegisterPanicsOnInvalidAndDuplicate(t *testing.T) {
	old := commands
	commands = map[string]Command{}
	defer func() { commands = old }()

	assertPanic(t, func() { Register(nil) })
	assertPanic(t, func() { Register(testCommand{}) })
	Register(testCommand{name: "alpha"})
	assertPanic(t, func() { Register(testCommand{name: "alpha"}) })
}

func assertPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}
