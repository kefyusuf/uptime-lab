package domain

import (
	"fmt"
	"net/url"
)

// TargetURL is the validated HTTP(S) target registered for a monitor.
//
// Validation here is syntactic product validation only. It does not establish
// that the target is safe to access from the future execution plane.
type TargetURL struct {
	raw string
}

// NewTargetURL validates a monitor target while preserving the accepted input.
func NewTargetURL(raw string) (TargetURL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return TargetURL{}, fmt.Errorf("%w: %v", ErrInvalidTargetURL, err)
	}

	if !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return TargetURL{}, ErrInvalidTargetURL
	}
	if parsed.Hostname() == "" {
		return TargetURL{}, ErrInvalidTargetURL
	}
	if parsed.User != nil {
		return TargetURL{}, ErrInvalidTargetURL
	}
	if parsed.Fragment != "" {
		return TargetURL{}, ErrInvalidTargetURL
	}

	return TargetURL{raw: raw}, nil
}

// String returns the exact accepted target string.
func (target TargetURL) String() string {
	return target.raw
}

func (target TargetURL) isZero() bool {
	return target.raw == ""
}
