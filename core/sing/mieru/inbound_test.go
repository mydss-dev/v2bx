package mieru

import (
	"testing"

	"github.com/sagernet/sing-box/option"
)

func TestBuildServerConfig(t *testing.T) {
	config, err := buildServerConfig(Options{
		ListenOptions: option.ListenOptions{ListenPort: 2999},
		Transport:     "TCP",
		MTU:           1400,
		Users: []User{{
			Name:     "test-user",
			Password: "test-password",
		}},
		UserHintIsMandatory: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := config.Config.GetPortBindings()[0].GetPort(); got != 2999 {
		t.Fatalf("port = %d, want 2999", got)
	}
	if got := config.Config.GetUsers()[0].GetName(); got != "test-user" {
		t.Fatalf("username = %q, want test-user", got)
	}
	if got := config.Config.GetMtu(); got != 1400 {
		t.Fatalf("MTU = %d, want 1400", got)
	}
	if !config.Config.GetAdvancedSettings().GetUserHintIsMandatory() {
		t.Fatal("user hint should be mandatory")
	}
}

func TestValidateOptionsRejectsInvalidTransport(t *testing.T) {
	err := validateOptions(Options{
		ListenOptions: option.ListenOptions{ListenPort: 2999},
		Transport:     "QUIC",
		Users:         []User{{Name: "user", Password: "password"}},
	})
	if err == nil {
		t.Fatal("expected invalid transport error")
	}
}
