package application

import "errors"

var (
	// ErrInvalidInventoryQuery is the stable invalid inventory input result.
	ErrInvalidInventoryQuery = errors.New("invalid monitor inventory query")
	// ErrAvailabilityEvaluation is the stable invalid assessment or clock failure.
	ErrAvailabilityEvaluation = errors.New("availability evaluation failed")
	// ErrMonitorNotFound is the stable application-level not-found result.
	ErrMonitorNotFound = errors.New("monitor not found")

	// ErrNoTerminalCheckResult is the stable application-level known-Monitor-without-result outcome.
	ErrNoTerminalCheckResult = errors.New("no terminal check result")

	// ErrNoDueCheck is the stable application-level no-work result.
	ErrNoDueCheck = errors.New("no due check")

	// ErrCheckRunNotFound is the stable application-level unknown CheckRun result.
	ErrCheckRunNotFound = errors.New("check run not found")

	// ErrCheckRunConflict is the stable application-level terminalization conflict result.
	ErrCheckRunConflict = errors.New("check run conflict")

	// ErrPersistence is the stable application-level persistence failure.
	ErrPersistence = errors.New("monitor persistence failed")
)
