package config

import "testing"

func TestInjectMethodFor(t *testing.T) {
	c := &Config{AppRules: map[string]AppRule{
		"com.example.paste": {InjectMethod: "paste"},
		"com.example.clip":  {InjectMethod: "clipboard"},
		"com.example.type":  {InjectMethod: "type"},
	}}
	cases := map[string]string{
		"":                  "paste",
		"com.example.none":  "paste",
		"com.example.paste": "paste",
		"com.example.clip":  "clipboard",
		"com.example.type":  "type",
	}
	for id, want := range cases {
		if got := c.InjectMethodFor(id); got != want {
			t.Errorf("InjectMethodFor(%q) = %q, want %q", id, got, want)
		}
	}
}
