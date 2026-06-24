package contracts

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunScheduledJobsRunsOnceJobs(t *testing.T) {
	var ran []string
	err := RunScheduledJobs(context.Background(), []ScheduledJob{
		{Name: "first", Schedule: "@once", Run: func(context.Context) error {
			ran = append(ran, "first")
			return nil
		}},
		{Name: "second", Schedule: "@once", Run: func(context.Context) error {
			ran = append(ran, "second")
			return nil
		}},
	})
	if err != nil {
		t.Fatalf("RunScheduledJobs: %v", err)
	}
	if strings.Join(ran, ",") != "first,second" {
		t.Fatalf("unexpected run order: %#v", ran)
	}
}

func TestValidateScheduledJobsRejectsUnsupportedSchedule(t *testing.T) {
	err := ValidateScheduledJobs([]ScheduledJob{{
		Name:     "sync",
		Schedule: "* * * * *",
		Run:      func(context.Context) error { return nil },
	}})
	if err == nil || !strings.Contains(err.Error(), "expected @once or @every <duration>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunScheduledJobsWrapsJobError(t *testing.T) {
	want := errors.New("down")
	err := RunScheduledJobs(context.Background(), []ScheduledJob{{
		Name:     "sync",
		Schedule: "@once",
		Run:      func(context.Context) error { return want },
	}})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), `scheduled job "sync" failed`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
