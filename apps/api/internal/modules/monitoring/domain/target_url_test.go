package domain

import (
	"errors"
	"testing"
)

func TestNewTargetURLAcceptsSupportedAbsoluteURLs(t *testing.T) {
	tests := []string{
		"http://example.com",
		"https://example.com/path?x=1",
		"https://example.com:8443/path",
		"HTTPS://Example.COM/path",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			target, err := NewTargetURL(raw)
			if err != nil {
				t.Fatalf("NewTargetURL(%q) error = %v", raw, err)
			}
			if target.String() != raw {
				t.Fatalf("TargetURL.String() = %q, want original %q", target.String(), raw)
			}
		})
	}
}

func TestNewTargetURLRejectsInvalidTargets(t *testing.T) {
	tests := map[string]string{
		"missing scheme":     "example.com/path",
		"unsupported scheme": "ftp://example.com/file",
		"missing hostname":   "https:///path",
		"embedded userinfo":  "https://user:secret@example.com/path",
		"fragment":           "https://example.com/path#fragment",
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewTargetURL(raw)
			if !errors.Is(err, ErrInvalidTargetURL) {
				t.Fatalf("NewTargetURL(%q) error = %v, want ErrInvalidTargetURL", raw, err)
			}
		})
	}
}

func TestNewTargetURLPreservesAcceptedInputWithoutLossyNormalization(t *testing.T) {
	raw := "https://Example.COM:8443/a/../b?x=1%202"
	target, err := NewTargetURL(raw)
	if err != nil {
		t.Fatalf("NewTargetURL() error = %v", err)
	}
	if got := target.String(); got != raw {
		t.Fatalf("TargetURL.String() = %q, want %q", got, raw)
	}
}
