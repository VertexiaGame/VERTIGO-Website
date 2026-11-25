package models

import "testing"

func TestCompactCount(t *testing.T) {
	cases := map[int]string{
		-5:         "0",
		0:          "0",
		999:        "999",
		1000:       "1K",
		1234:       "1.2K",
		9999:       "9.9K",
		10_000:     "10K",
		345_678:    "345K",
		1_000_000:  "1M",
		5_670_000:  "5.6M",
		12_000_000: "12M",
	}
	for in, want := range cases {
		if got := CompactCount(in); got != want {
			t.Errorf("CompactCount(%d) = %q, want %q", in, got, want)
		}
	}
}
