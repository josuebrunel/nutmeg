// Logging helpers shared by Nutmeg's River job workers. River doesn't surface
// job lifecycle with the structured fields we want by itself, so each Work
// method reports its own start and outcome through these — a completion is
// logged at Info (visible without DEBUG) and failures at Error, while per-job
// detail beyond that is left for the caller to pass as extra key/values.
package worker

import (
	"log/slog"
	"time"

	"github.com/riverqueue/river"
)

// logJobStarted reports that a background job's Work method has begun.
func logJobStarted[T river.JobArgs](job *river.Job[T], extra ...any) {
	attrs := append([]any{"kind", job.Kind, "job_id", job.ID, "attempt", job.Attempt}, extra...)
	slog.Info("job started", attrs...)
}

// logJobOutcome reports a job's completion (Info) or failure (Error) along
// with its elapsed runtime, so a job's full lifecycle is traceable without
// enabling Debug.
func logJobOutcome[T river.JobArgs](job *river.Job[T], startedAt time.Time, err error, extra ...any) {
	attrs := append([]any{"kind", job.Kind, "job_id", job.ID, "attempt", job.Attempt, "duration", time.Since(startedAt)}, extra...)
	if err != nil {
		slog.Error("job failed", append(attrs, "error", err)...)
		return
	}
	slog.Info("job completed", attrs...)
}
