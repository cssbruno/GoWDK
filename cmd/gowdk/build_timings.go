package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const buildTimingsFile = "gowdk-build-timings.json"

type buildTimingRecorder struct {
	enabled  bool
	phases   []buildTimingPhase
	counters map[string]int
}

type buildTimingReport struct {
	Version   int                `json:"version"`
	Mode      string             `json:"mode"`
	OutputDir string             `json:"outputDir"`
	Phases    []buildTimingPhase `json:"phases"`
	Counters  map[string]int     `json:"counters,omitempty"`
}

type buildTimingPhase struct {
	Name       string  `json:"name"`
	DurationMS float64 `json:"durationMs"`
}

func newBuildTimingRecorder(enabled bool) *buildTimingRecorder {
	return &buildTimingRecorder{
		enabled:  enabled,
		counters: map[string]int{},
	}
}

func (recorder *buildTimingRecorder) clone() *buildTimingRecorder {
	if recorder == nil {
		return nil
	}
	clone := newBuildTimingRecorder(recorder.enabled)
	clone.phases = append(clone.phases, recorder.phases...)
	for key, value := range recorder.counters {
		clone.counters[key] = value
	}
	return clone
}

func (recorder *buildTimingRecorder) addDuration(name string, duration time.Duration) {
	if recorder == nil || !recorder.enabled {
		return
	}
	recorder.phases = append(recorder.phases, buildTimingPhase{
		Name:       name,
		DurationMS: float64(duration.Microseconds()) / 1000,
	})
}

func (recorder *buildTimingRecorder) measure(name string, fn func() error) error {
	if recorder == nil || !recorder.enabled {
		return fn()
	}
	start := time.Now()
	err := fn()
	recorder.addDuration(name, time.Since(start))
	return err
}

func (recorder *buildTimingRecorder) counter(name string, value int) {
	if recorder == nil || !recorder.enabled {
		return
	}
	recorder.counters[name] += value
}

func (recorder *buildTimingRecorder) write(outputDir string, explicitPath string) (string, error) {
	if recorder == nil || !recorder.enabled {
		return "", nil
	}
	path := strings.TrimSpace(explicitPath)
	if path == "" {
		path = filepath.Join(outputDir, buildTimingsFile)
	}
	report := buildTimingReport{
		Version:   1,
		Mode:      "build",
		OutputDir: outputDir,
		Phases:    append([]buildTimingPhase(nil), recorder.phases...),
		Counters:  sortedTimingCounters(recorder.counters),
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sortedTimingCounters(counters map[string]int) map[string]int {
	if len(counters) == 0 {
		return nil
	}
	keys := make([]string, 0, len(counters))
	for key := range counters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sorted := map[string]int{}
	for _, key := range keys {
		sorted[key] = counters[key]
	}
	return sorted
}

func parseBuildTimingFlagValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("timings output path is required")
	}
	return value, nil
}
