package domain

import "errors"

var (
	ErrInvalidMonitorID   = errors.New("invalid monitor id")
	ErrInvalidCheckID     = errors.New("invalid check id")
	ErrInvalidCheckResult = errors.New("invalid check result")
	ErrInvalidTargetURL   = errors.New("invalid target URL")
	ErrInvalidCreatedAt   = errors.New("invalid monitor creation time")
)
