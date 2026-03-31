package updater

import (
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	if compareVersions("1.3.2", "1.3.2") != 0 {
		t.Fatal("expected equal versions")
	}
	if compareVersions("1.3.3", "1.3.2") <= 0 {
		t.Fatal("expected newer version to compare greater")
	}
	if compareVersions("v1.3.1", "1.3.2") >= 0 {
		t.Fatal("expected older version to compare lower")
	}
}

func TestSelectAssets(t *testing.T) {
	asset, checksum := selectAssets([]githubAsset{
		{Name: "foundry-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz", BrowserDownloadURL: "https://example.com/foundry.tar.gz"},
		{Name: "foundry-" + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz.sha256", BrowserDownloadURL: "https://example.com/foundry.tar.gz.sha256"},
	})
	if asset == nil || checksum == nil {
		t.Fatalf("expected asset and checksum, got %#v %#v", asset, checksum)
	}
}
