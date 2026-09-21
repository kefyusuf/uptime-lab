package domain

import "errors"

var (
	ErrInvalidMonitorID = errors.New("invalid monitor id")
	ErrInvalidTargetURL = errors.New("invalid target URL")
	ErrInvalidCreatedAt = errors.New("invalid monitor creation time")
)
