package application

import (
	"context"
	"errors"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/domain"
	"github.com/kefyusuf/uptime-lab/apps/api/internal/modules/monitoring/ports"
	"time"
)

// MonitorAvailability is the immutable, public-safe assessment of one read.
type MonitorAvailability struct {
	assessment  domain.Availability
	evaluatedAt time.Time
	evidence    MonitorAvailabilityEvidence
	hasEvidence bool
}

// MonitorAvailabilityEvidence links the assessment to its terminal source fact.
type MonitorAvailabilityEvidence struct {
	CheckID     domain.CheckID
	CompletedAt time.Time
}

func (a MonitorAvailability) Status() domain.AvailabilityStatus { return a.assessment.Status() }
func (a MonitorAvailability) Reason() domain.AvailabilityReason { return a.assessment.Reason() }
func (a MonitorAvailability) EvaluatedAt() time.Time            { return a.evaluatedAt }
func (a MonitorAvailability) Evidence() (MonitorAvailabilityEvidence, bool) {
	return a.evidence, a.hasEvidence
}

// GetMonitorAvailability derives current availability without mutating durable state.
type GetMonitorAvailability struct {
	repository ports.LatestCheckResultRepository
	clock      Clock
}

func NewGetMonitorAvailability(repository ports.LatestCheckResultRepository, clock Clock) GetMonitorAvailability {
	return GetMonitorAvailability{repository: repository, clock: clock}
}

func (useCase GetMonitorAvailability) Execute(ctx context.Context, monitorID domain.MonitorID) (MonitorAvailability, error) {
	if _, err := domain.NewMonitorID(monitorID.UUID()); err != nil {
		return MonitorAvailability{}, err
	}
	records, err := useCase.repository.RecentTerminalByMonitorID(ctx, monitorID, domain.AvailabilitySequenceLimit)
	if err != nil {
		if errors.Is(err, ports.ErrMonitorNotFound) {
			return MonitorAvailability{}, ErrMonitorNotFound
		}
		return MonitorAvailability{}, ErrPersistence
	}
	observations := make([]domain.AvailabilityObservation, 0, len(records))
	var evidence MonitorAvailabilityEvidence
	for index, record := range records {
		result, validationErr := latestCheckResultFromRecord(record)
		if validationErr != nil || !availabilityTimestampSafe(result.CompletedAt()) {
			return MonitorAvailability{}, ErrPersistence
		}
		observation := domain.AvailabilityObservation{Kind: result.ResultKind(), CompletedAt: result.CompletedAt()}
		if status, ok := result.HTTPStatus(); ok {
			observation.HTTPStatus = &status
		}
		observations = append(observations, observation)
		if index == 0 {
			evidence = MonitorAvailabilityEvidence{CheckID: result.CheckID(), CompletedAt: result.CompletedAt()}
		}
	}
	if useCase.clock == nil {
		return MonitorAvailability{}, ErrAvailabilityEvaluation
	}
	evaluatedAt := useCase.clock().UTC()
	if !availabilityTimestampSafe(evaluatedAt) {
		return MonitorAvailability{}, ErrAvailabilityEvaluation
	}
	assessment, err := domain.EvaluateAvailability(observations, evaluatedAt)
	if err != nil {
		return MonitorAvailability{}, ErrAvailabilityEvaluation
	}
	return MonitorAvailability{assessment: assessment, evaluatedAt: evaluatedAt, evidence: evidence, hasEvidence: len(observations) > 0}, nil
}

func availabilityTimestampSafe(value time.Time) bool {
	return !value.IsZero() && value.UTC().Year() >= 0 && value.UTC().Year() <= 9999
}
