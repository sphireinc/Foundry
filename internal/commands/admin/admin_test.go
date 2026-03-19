package admincmd

import "testing"

func TestAdminCommandMetadataAndRun(t *testing.T) {
	cmd := command{}
	if cmd.Name() != "admin" || cmd.Group() == "" || cmd.RequiresConfig() {
		t.Fatalf("unexpected admin command metadata")
	}
	if len(cmd.Details()) != 2 {
		t.Fatalf("unexpected admin command details: %#v", cmd.Details())
	}
	if err := cmd.Run(nil, []string{"foundry", "admin", "hash-password", "secret-password"}); err != nil {
		t.Fatalf("hash-password run: %v", err)
	}
	if err := cmd.Run(nil, []string{"foundry", "admin", "sample-user", "admin", "Admin User", "admin@example.com", "secret-password"}); err != nil {
		t.Fatalf("sample-user run: %v", err)
	}
	if err := cmd.Run(nil, []string{"foundry", "admin"}); err == nil {
		t.Fatal("expected missing admin subcommand error")
	}
	if err := cmd.Run(nil, []string{"foundry", "admin", "nope"}); err == nil {
		t.Fatal("expected unknown admin subcommand error")
	}
}
