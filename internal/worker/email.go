package worker

import (
	"context"
	"log/slog"

	"github.com/riverqueue/river"
)

// EmailSender is satisfied by *email.Client — declared here rather than
// imported so this file doesn't need to know about SMTP configuration.
type EmailSender interface {
	Send(ctx context.Context, to []string, subject, body string) error
}

// SendEmailArgs enqueues a single outbound email. Routed through River
// (rather than sent inline by the handler) so a slow or down mail relay
// never blocks the request that triggered it, and gets River's normal
// retry/backoff on failure.
type SendEmailArgs struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// Kind satisfies river.JobArgs.
func (SendEmailArgs) Kind() string { return "send_email" }

type SendEmailWorker struct {
	river.WorkerDefaults[SendEmailArgs]
	Client EmailSender
}

func (w *SendEmailWorker) Work(ctx context.Context, job *river.Job[SendEmailArgs]) error {
	if err := w.Client.Send(ctx, job.Args.To, job.Args.Subject, job.Args.Body); err != nil {
		slog.Error("send email failed", "to", job.Args.To, "subject", job.Args.Subject, "error", err)
		return err
	}
	slog.Debug("sent email", "to", job.Args.To, "subject", job.Args.Subject)
	return nil
}
