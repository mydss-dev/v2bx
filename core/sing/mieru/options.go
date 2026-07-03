package mieru

import "github.com/sagernet/sing-box/option"

type Options struct {
	option.ListenOptions
	Users               []User `json:"users,omitempty"`
	Transport           string `json:"transport,omitempty"`
	TrafficPattern      string `json:"traffic_pattern,omitempty"`
	MTU                 int32  `json:"mtu,omitempty"`
	UserHintIsMandatory bool   `json:"user_hint_is_mandatory,omitempty"`
}

type User struct {
	Name     string `json:"name,omitempty"`
	Password string `json:"password,omitempty"`
}
