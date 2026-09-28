package application

import "errors"

var (
	// ErrMonitorNotFound is the stable application-level not-found result.
	ErrMonitorNotFound = errors.New("monitor not found")

	// ErrNoDueCheck is the stable application-level no-work result.
	ErrNoDueCheck = errors.New("no due check")

	// ErrCheckRunNotFound is the stable application-level unknown CheckRun result.
	ErrCheckRunNotFound = errors.New("check run not found")

	// ErrCheckRunConflict is the stable application-level terminalization conflict result.
	ErrCheckRunConflict = errors.New("check run conflict")

	// ErrPersistence is the stable application-level persistence failure.
	ErrPersistence = errors.New("monitor persistence failed")
)
