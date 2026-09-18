package config

import (
	"reflect"
	"testing"
)

func TestParseOrigins(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty falls back to default", "", defaultAllowedOrigins},
		{"single origin", "https://example.com", []string{"https://example.com"}},
		{"multiple comma-separated", "https://a.com,https://b.com", []string{"https://a.com", "https://b.com"}},
		{"trims whitespace around entries", " https://a.com , https://b.com ", []string{"https://a.com", "https://b.com"}},
		{"ignores empty entries from trailing commas", "https://a.com,,", []string{"https://a.com"}},
		{"only commas falls back to default", " , , ", defaultAllowedOrigins},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseOrigins(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseOrigins(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
