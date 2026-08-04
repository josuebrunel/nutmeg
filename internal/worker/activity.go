package worker

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"

	"nutmeg/internal/service"
)

// GenerateGroupNewsArgs enqueues an AI upgrade for one group_activity row,
// created synchronously (with real fallback content already in place) by
// the handler that triggered the event.
type GenerateGroupNewsArgs struct {
	ActivityID string `json:"activity_id"`
	EventKind  string `json:"event_kind"`
	SubjectID  string `json:"subject_id"`
}

// Kind satisfies river.JobArgs.
func (GenerateGroupNewsArgs) Kind() string { return "generate_group_news" }

type GenerateGroupNewsWorker struct {
	river.WorkerDefaults[GenerateGroupNewsArgs]
	Service *service.ActivityService
}

func (w *GenerateGroupNewsWorker) Work(ctx context.Context, job *river.Job[GenerateGroupNewsArgs]) error {
	if err := w.Service.GenerateNews(ctx, job.Args.ActivityID, job.Args.EventKind, job.Args.SubjectID); err != nil {
		// Same discipline as commentary generation: log and let River's
		// retry/backoff take over. The activity row already has usable
		// fallback content, so a failure here never leaves a blank feed.
		slog.Error("generate group news failed", "activity_id", job.Args.ActivityID, "error", err)
		return err
	}
	slog.Debug("generated group news", "activity_id", job.Args.ActivityID)
	return nil
}
