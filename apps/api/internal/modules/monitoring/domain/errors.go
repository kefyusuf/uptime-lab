package domain

import "errors"

var (
	ErrInvalidSchedulingState = errors.New("invalid scheduling state")
	ErrInvalidAvailability    = errors.New("invalid availability assessment")
	ErrInvalidMonitorID       = errors.New("invalid monitor id")
	ErrInvalidCheckID         = errors.New("invalid check id")
	ErrInvalidCheckResult     = errors.New("invalid check result")
	ErrInvalidTargetURL       = errors.New("invalid target URL")
	ErrInvalidCreatedAt       = errors.New("invalid monitor creation time")
)
