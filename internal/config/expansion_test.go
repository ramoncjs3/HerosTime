package config

import (
	"path/filepath"
	"testing"
)

func TestExpansionAddPersistLoadAndRemove(t *testing.T) {
	file := filepath.Join(t.TempDir(), "expansion.json")
	cfg := testExpansionConfig(file)

	record, err := cfg.AddExpansionAccount(ExpansionAccountInput{
		VariantName:     "官方",
		Server:          "g99",
		WxPusherTopicID: 10099,
		QQGroupID:       "9988",
	})
	if err != nil {
		t.Fatalf("AddExpansionAccount: %v", err)
	}
	if record.Kind != "official" || record.Server != "g99" {
		t.Fatalf("record=%+v", record)
	}
	variants := cfg.EnabledVariants()
	if len(variants) != 1 || len(variants[0].Accounts) != 1 || variants[0].Accounts[0].Server != "g99" {
		t.Fatalf("variants=%+v", variants)
	}
	if cfg.WxPusher.TopicMap["g99"] != 10099 || cfg.QQ.GroupMap["g99"] != "9988" {
		t.Fatalf("destination maps not updated: wx=%v qq=%v", cfg.WxPusher.TopicMap, cfg.QQ.GroupMap)
	}

	loaded := testExpansionConfig(file)
	if err := loaded.loadExpansionFile(); err != nil {
		t.Fatalf("loadExpansionFile: %v", err)
	}
	loadedVariants := loaded.EnabledVariants()
	if len(loadedVariants[0].Accounts) != 1 || loadedVariants[0].Accounts[0].Server != "g99" {
		t.Fatalf("loaded variants=%+v", loadedVariants)
	}
	if loaded.WxPusher.TopicMap["g99"] != 10099 || loaded.QQ.GroupMap["g99"] != "9988" {
		t.Fatalf("loaded destination maps not updated: wx=%v qq=%v", loaded.WxPusher.TopicMap, loaded.QQ.GroupMap)
	}

	if _, err := loaded.RemoveExpansionAccount(ExpansionAccountInput{VariantName: "官方", Server: "g99"}); err != nil {
		t.Fatalf("RemoveExpansionAccount: %v", err)
	}
	if len(loaded.EnabledVariants()[0].Accounts) != 0 {
		t.Fatalf("server was not removed: %+v", loaded.EnabledVariants()[0].Accounts)
	}
	if _, ok := loaded.WxPusher.TopicMap["g99"]; ok {
		t.Fatalf("wxpusher topic was not removed: %v", loaded.WxPusher.TopicMap)
	}
	if _, ok := loaded.QQ.GroupMap["g99"]; ok {
		t.Fatalf("qq group was not removed: %v", loaded.QQ.GroupMap)
	}
}

func TestExpansionDuplicateServer(t *testing.T) {
	cfg := testExpansionConfig(filepath.Join(t.TempDir(), "expansion.json"))
	if _, err := cfg.AddExpansionAccount(ExpansionAccountInput{VariantName: "官方", Server: "g99"}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := cfg.AddExpansionAccount(ExpansionAccountInput{VariantName: "官方", Server: "g99"}); err == nil {
		t.Fatalf("duplicate add returned nil error")
	}
}

func testExpansionConfig(file string) *Config {
	enabled := true
	return &Config{
		Expansion: ExpansionConfig{File: file},
		QQ:        QQConfig{GroupMap: map[string]string{}},
		WxPusher:  WxPusherConfig{TopicMap: map[string]int64{}},
		Variants: []Variant{
			{
				Name:    "官方",
				Kind:    "official",
				Enabled: &enabled,
				Auth: AuthConfig{
					Username: "user",
					Password: "pass",
				},
				ServerRules: []ServerRule{{MatchPrefix: "g", CodePrefix: "g"}},
			},
		},
	}
}
