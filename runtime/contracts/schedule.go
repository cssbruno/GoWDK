package contracts

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// CronOverlapSkip skips a scheduled tick when the previous run is still active.
	CronOverlapSkip = "skip"
	// CronMissedRunSkip drops missed ticks instead of replaying catch-up runs.
	CronMissedRunSkip = "skip"
)

// ScheduledJob is one dependency-free generated cron role job.
type ScheduledJob struct {
	Name            string
	Schedule        string
	OverlapPolicy   string
	MissedRunPolicy string
	Run             func(context.Context) error
}

type parsedScheduledJob struct {
	Job      ScheduledJob
	Once     bool
	Interval time.Duration
}

// ValidateScheduledJobs validates the generated scheduled-job contract without
// running any handlers.
func ValidateScheduledJobs(jobs []ScheduledJob) error {
	_, err := parseScheduledJobs(jobs)
	return err
}

// RunScheduledJobs runs dependency-free scheduled jobs until all @once jobs
// complete, ctx is canceled, or a job returns an error.
func RunScheduledJobs(ctx context.Context, jobs []ScheduledJob) error {
	if ctx == nil {
		ctx = context.Background()
	}
	parsed, err := parseScheduledJobs(jobs)
	if err != nil {
		return err
	}
	if len(parsed) == 0 {
		return fmt.Errorf("scheduled job runner requires at least one job")
	}
	if allOnceSchedules(parsed) {
		for _, job := range parsed {
			if err := runScheduledJob(ctx, job.Job); err != nil {
				return err
			}
		}
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, len(parsed))
	var wait sync.WaitGroup
	for _, job := range parsed {
		job := job
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := runScheduledJobLoop(runCtx, job); err != nil {
				select {
				case errs <- err:
				default:
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case err := <-errs:
		cancel()
		<-done
		return err
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	case <-done:
		return nil
	}
}

func parseScheduledJobs(jobs []ScheduledJob) ([]parsedScheduledJob, error) {
	out := make([]parsedScheduledJob, 0, len(jobs))
	for index, job := range jobs {
		parsed, err := parseScheduledJob(index, job)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func parseScheduledJob(index int, job ScheduledJob) (parsedScheduledJob, error) {
	name := strings.TrimSpace(job.Name)
	if name == "" {
		return parsedScheduledJob{}, fmt.Errorf("scheduled job %d is missing name", index)
	}
	if job.Run == nil {
		return parsedScheduledJob{}, fmt.Errorf("scheduled job %q is missing Run hook", name)
	}
	overlap := strings.TrimSpace(job.OverlapPolicy)
	if overlap == "" {
		overlap = CronOverlapSkip
	}
	if overlap != CronOverlapSkip {
		return parsedScheduledJob{}, fmt.Errorf("scheduled job %q has unsupported overlap policy %q; expected %q", name, job.OverlapPolicy, CronOverlapSkip)
	}
	missed := strings.TrimSpace(job.MissedRunPolicy)
	if missed == "" {
		missed = CronMissedRunSkip
	}
	if missed != CronMissedRunSkip {
		return parsedScheduledJob{}, fmt.Errorf("scheduled job %q has unsupported missed-run policy %q; expected %q", name, job.MissedRunPolicy, CronMissedRunSkip)
	}
	schedule := strings.TrimSpace(job.Schedule)
	switch {
	case schedule == "@once":
		return parsedScheduledJob{Job: job, Once: true}, nil
	case strings.HasPrefix(schedule, "@every "):
		value := strings.TrimSpace(strings.TrimPrefix(schedule, "@every "))
		interval, err := time.ParseDuration(value)
		if err != nil || interval <= 0 {
			if err == nil {
				err = fmt.Errorf("duration must be positive")
			}
			return parsedScheduledJob{}, fmt.Errorf("scheduled job %q has invalid @every schedule %q: %w", name, schedule, err)
		}
		return parsedScheduledJob{Job: job, Interval: interval}, nil
	default:
		return parsedScheduledJob{}, fmt.Errorf("scheduled job %q has unsupported schedule %q; expected @once or @every <duration>", name, job.Schedule)
	}
}

func allOnceSchedules(jobs []parsedScheduledJob) bool {
	for _, job := range jobs {
		if !job.Once {
			return false
		}
	}
	return true
}

func runScheduledJobLoop(ctx context.Context, job parsedScheduledJob) error {
	if job.Once {
		return runScheduledJob(ctx, job.Job)
	}
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	var running bool
	done := make(chan error, 1)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			running = false
			if err != nil {
				return err
			}
		case <-ticker.C:
			if running {
				continue
			}
			running = true
			go func() {
				done <- runScheduledJob(ctx, job.Job)
			}()
		}
	}
}

func runScheduledJob(ctx context.Context, job ScheduledJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := job.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("scheduled job %q failed: %w", job.Name, err)
	}
	return nil
}
