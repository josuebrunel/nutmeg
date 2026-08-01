// Package worker holds Nutmeg's RiverQueue job definitions. This is the
// app's first background job — introduced specifically to generate player
// roast commentary asynchronously after a match is logged, so a slow local
// LLM call never blocks the match-save request.
package worker

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"nutmeg/internal/service"
)

// GenerateCommentaryArgs enqueues a roast-commentary generation for one
// player. MatchID is set when a newly logged match triggered the job, and
// left nil for an admin-triggered manual regeneration.
//
// GroupPlayerID carries `river:"unique"` so InsertUniqueOpts (below) can
// scope uniqueness to it alone — MatchID must stay outside the uniqueness
// hash so an auto-triggered job and a manual regeneration for the same
// player still dedupe against each other.
type GenerateCommentaryArgs struct {
	GroupPlayerID string  `json:"group_player_id" river:"unique"`
	MatchID       *string `json:"match_id,omitempty"`
}

// Kind satisfies river.JobArgs.
func (GenerateCommentaryArgs) Kind() string { return "generate_commentary" }

// InsertOpts satisfies river.JobArgsWithInsertOpts, applying to every job
// of this kind by default. It prevents a second generation job for the
// same player from being enqueued while one is still in flight (available,
// pending, running, retryable, or scheduled) — without this, rapidly
// re-clicking "Regenerate" before the first job finishes would bypass the
// cooldown, since that check only looks at the last *completed*
// generation. Completed jobs are deliberately excluded from the uniqueness
// window so a genuinely new generation remains enqueueable afterwards.
func (GenerateCommentaryArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

type GenerateCommentaryWorker struct {
	river.WorkerDefaults[GenerateCommentaryArgs]
	Service *service.CommentaryService
}

func (w *GenerateCommentaryWorker) Work(ctx context.Context, job *river.Job[GenerateCommentaryArgs]) error {
	if err := w.Service.Generate(ctx, job.Args.GroupPlayerID, job.Args.MatchID); err != nil {
		// Generation failures (LLM unreachable, output failed validation)
		// are logged and left to River's normal retry/backoff — they must
		// never take down match creation, which already succeeded by the
		// time this job runs.
		slog.Error("generate commentary failed", "group_player_id", job.Args.GroupPlayerID, "error", err)
		return err
	}
	return nil
}
