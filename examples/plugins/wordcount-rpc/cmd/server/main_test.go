package main

import (
	"github.com/sphireinc/foundry/sdk/pluginrpc"
	"testing"
)

func TestWordCount(t *testing.T) {
	for _, tc := range []struct {
		page  *pluginrpc.PagePayload
		words int
	}{{nil, 0}, {&pluginrpc.PagePayload{RawBody: "one two three"}, 3}} {
		result, err := (handler{}).Context(pluginrpc.ContextRequest{Page: tc.page})
		if err != nil {
			t.Fatal(err)
		}
		if got := result.Data["wordcount_rpc"].(map[string]any)["words"]; got != tc.words {
			t.Fatalf("got %v want %d", got, tc.words)
		}
	}
}
