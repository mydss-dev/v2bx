package panel

import (
	"testing"

	"github.com/InazumaV/V2bX/conf"
)

func TestNewAcceptsMieruNodeType(t *testing.T) {
	client, err := New(&conf.ApiConfig{
		APIHost:  "http://127.0.0.1",
		Key:      "token",
		NodeType: "Mieru",
		NodeID:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.NodeType != "mieru" {
		t.Fatalf("node type = %q, want mieru", client.NodeType)
	}
}
