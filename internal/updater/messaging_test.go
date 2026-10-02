package updater

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	versioncmd "github.com/sphireinc/foundry/internal/commands/version"
)

type releaseTransport func(*http.Request) (*http.Response, error)

func (fn releaseTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestCheckBuildProvenance(t *testing.T) {
	oldClient, oldTag, oldVersion := http.DefaultClient, versioncmd.BuildTag, versioncmd.Version
	t.Cleanup(func() { http.DefaultClient, versioncmd.BuildTag, versioncmd.Version = oldClient, oldTag, oldVersion })
	t.Setenv("FOUNDRY_CONTAINER", "true")
	http.DefaultClient = &http.Client{Transport: releaseTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v1.5.0","html_url":"https://example.com/release"}`))}, nil
	})}
	versioncmd.Version = "v1.4.6"
	for _, tag := range []string{"", "v1.4.6"} {
		versioncmd.BuildTag = tag
		info, err := Check(context.Background(), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if info.HasUpdate != (tag != "") || info.ReleaseComparable != (tag != "") || info.ApplySupported {
			t.Fatalf("incorrect container comparison: %+v", info)
		}
		if !strings.Contains(info.Instructions, "recreate") {
			t.Fatal(info.Instructions)
		}
	}
}

func TestCustomStandaloneInstructions(t *testing.T) {
	got := instructionsForMode(ModeStandalone, versioncmd.Metadata{BuildKind: "modified_build"})
	if !strings.Contains(got, "disabled") {
		t.Fatal(got)
	}
}
