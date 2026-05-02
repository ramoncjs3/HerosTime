package wxpusher

import (
	"testing"

	"oldbeggar-refactor/internal/config"
)

func TestEnvServerCode(t *testing.T) {
	cases := map[string]string{
		"g1":    "G1",
		"h5_10": "H5_10",
		"b-2":   "B_2",
		" h 3 ": "_H_3_",
	}
	for input, want := range cases {
		if got := envServerCode(input); got != want {
			t.Fatalf("envServerCode(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestTopicIDPrefersEnvironment(t *testing.T) {
	t.Setenv("WXPUSHER_TOPIC_ID_G1", "200")
	client := NewClient(nil, config.WxPusherConfig{
		TopicMap: map[string]int64{"g1": 100},
	})
	if got := client.topicID("g1"); got != 200 {
		t.Fatalf("topicID()=%d, want 200", got)
	}
}

func TestLimitRunes(t *testing.T) {
	if got := limitRunes("铁剑令提醒ABCDE", 4); got != "铁剑令提" {
		t.Fatalf("limitRunes()=%q", got)
	}
}
