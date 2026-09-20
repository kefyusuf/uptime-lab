package application

import "errors"

var (
	// ErrMonitorNotFound is the stable application-level not-found result.
	ErrMonitorNotFound = errors.New("monitor not found")

	// ErrPersistence is the stable application-level persistence failure.
	ErrPersistence = errors.New("monitor persistence failed")
)
