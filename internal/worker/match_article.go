package worker

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"

	"nutmeg/internal/service"
)

// GenerateMatchArticleArgs enqueues an AI upgrade for one match_article
// row, created synchronously (with fallback content already in place) by
// MatchHandler.Create right after a match is logged, or by
// GroupHandler.RegenerateMatchArticle on an admin-triggered regeneration.
type GenerateMatchArticleArgs struct {
	ArticleID string `json:"article_id"`
	MatchID   string `json:"match_id"`
}

// Kind satisfies river.JobArgs.
func (GenerateMatchArticleArgs) Kind() string { return "generate_match_article" }

type GenerateMatchArticleWorker struct {
	river.WorkerDefaults[GenerateMatchArticleArgs]
	Service *service.MatchArticleService
}

func (w *GenerateMatchArticleWorker) Work(ctx context.Context, job *river.Job[GenerateMatchArticleArgs]) error {
	if err := w.Service.GenerateArticle(ctx, job.Args.ArticleID, job.Args.MatchID); err != nil {
		// Same discipline as commentary/group-news generation: log and let
		// River's retry/backoff take over. The article row already has
		// usable fallback (or prior) content, so a failure here never
		// leaves a blank view.
		slog.Error("generate match article failed", "article_id", job.Args.ArticleID, "match_id", job.Args.MatchID, "error", err)
		return err
	}
	return nil
}
