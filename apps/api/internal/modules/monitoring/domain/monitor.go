package domain

import "time"

// Monitor is the minimal immutable monitor aggregate for this foundation.
type Monitor struct {
	id        MonitorID
	targetURL TargetURL
	createdAt time.Time
}

// NewMonitor creates a valid monitor and normalizes its creation instant to UTC.
func NewMonitor(id MonitorID, targetURL TargetURL, createdAt time.Time) (Monitor, error) {
	if id.isZero() {
		return Monitor{}, ErrInvalidMonitorID
	}
	if targetURL.isZero() {
		return Monitor{}, ErrInvalidTargetURL
	}
	if createdAt.IsZero() {
		return Monitor{}, ErrInvalidCreatedAt
	}

	return Monitor{
		id:        id,
		targetURL: targetURL,
		createdAt: createdAt.UTC(),
	}, nil
}

// ID returns the monitor identity.
func (monitor Monitor) ID() MonitorID {
	return monitor.id
}

// TargetURL returns the registered target.
func (monitor Monitor) TargetURL() TargetURL {
	return monitor.targetURL
}

// CreatedAt returns the UTC creation instant.
func (monitor Monitor) CreatedAt() time.Time {
	return monitor.createdAt
}

// WithTargetURL returns a new immutable Monitor value with the same identity
// and creation instant and the validated replacement target. The original
// value is never mutated.
func (monitor Monitor) WithTargetURL(targetURL TargetURL) (Monitor, error) {
	if targetURL.isZero() {
		return Monitor{}, ErrInvalidTargetURL
	}
	return Monitor{
		id:        monitor.id,
		targetURL: targetURL,
		createdAt: monitor.createdAt,
	}, nil
}
