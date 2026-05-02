package qq

import "testing"

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
