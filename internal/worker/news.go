package worker

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"nutmeg/internal/service"
)

// GenerateGroupNewsArgs enqueues an AI upgrade for one group_news row,
// created synchronously (with real fallback content already in place) by
// the handler that triggered the event — a short signing blurb for
// "player_added", a full match report for "match_logged".
type GenerateGroupNewsArgs struct {
	NewsID    string `json:"news_id"`
	EventKind string `json:"event_kind"`
	SubjectID string `json:"subject_id"`
}

// Kind satisfies river.JobArgs.
func (GenerateGroupNewsArgs) Kind() string { return "generate_group_news" }

type GenerateGroupNewsWorker struct {
	river.WorkerDefaults[GenerateGroupNewsArgs]
	Service *service.NewsService
}

func (w *GenerateGroupNewsWorker) Work(ctx context.Context, job *river.Job[GenerateGroupNewsArgs]) error {
	logJobStarted(job, "news_id", job.Args.NewsID, "event_kind", job.Args.EventKind)
	started := time.Now()
	err := w.Service.GenerateNews(ctx, job.Args.NewsID, job.Args.EventKind, job.Args.SubjectID)
	// Same discipline as commentary generation: log and let River's
	// retry/backoff take over. The news row already has usable fallback
	// content, so a failure here never leaves a blank feed.
	logJobOutcome(job, started, err, "news_id", job.Args.NewsID, "event_kind", job.Args.EventKind)
	return err
}
