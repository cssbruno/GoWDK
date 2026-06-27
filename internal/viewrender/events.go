package viewrender

import (
	"fmt"
	"strconv"
	"strings"
)

// EventDirective is a parsed g:on:<event>[.<modifier>] directive.
type EventDirective struct {
	Event      string
	Prevent    bool
	Stop       bool
	Once       bool
	Capture    bool
	DebounceMS int
	ThrottleMS int
}

// ParseEventDirective validates and splits a g:on directive name.
func ParseEventDirective(name string) (EventDirective, error) {
	if !strings.HasPrefix(name, "g:on:") {
		return EventDirective{}, fmt.Errorf("event directive %q must start with g:on", name)
	}
	raw := strings.TrimPrefix(name, "g:on:")
	if raw == "" {
		return EventDirective{}, fmt.Errorf("g:on directive requires an event name")
	}
	parts := strings.Split(raw, ".")
	directive := EventDirective{Event: parts[0]}
	if !eventNamePattern.MatchString(directive.Event) {
		return EventDirective{}, fmt.Errorf("g:on directive has invalid event name %q", directive.Event)
	}
	seen := map[string]bool{}
	for _, modifier := range parts[1:] {
		if modifier == "" {
			return EventDirective{}, fmt.Errorf("g:on:%s has an empty event modifier", raw)
		}
		key := modifier
		if strings.HasPrefix(modifier, "debounce(") {
			key = "debounce"
		} else if strings.HasPrefix(modifier, "throttle(") {
			key = "throttle"
		}
		if seen[key] {
			return EventDirective{}, fmt.Errorf("g:on:%s repeats %s modifier", raw, key)
		}
		seen[key] = true
		switch modifier {
		case "prevent":
			directive.Prevent = true
		case "stop":
			directive.Stop = true
		case "once":
			directive.Once = true
		case "capture":
			directive.Capture = true
		default:
			if strings.HasPrefix(modifier, "debounce(") && strings.HasSuffix(modifier, ")") {
				ms, err := parseEventDurationMS(strings.TrimSuffix(strings.TrimPrefix(modifier, "debounce("), ")"))
				if err != nil {
					return EventDirective{}, fmt.Errorf("g:on:%s has invalid debounce duration: %w", raw, err)
				}
				directive.DebounceMS = ms
				continue
			}
			if strings.HasPrefix(modifier, "throttle(") && strings.HasSuffix(modifier, ")") {
				ms, err := parseEventDurationMS(strings.TrimSuffix(strings.TrimPrefix(modifier, "throttle("), ")"))
				if err != nil {
					return EventDirective{}, fmt.Errorf("g:on:%s has invalid throttle duration: %w", raw, err)
				}
				directive.ThrottleMS = ms
				continue
			}
			return EventDirective{}, fmt.Errorf("g:on:%s uses unsupported event modifier %q", raw, modifier)
		}
	}
	if directive.DebounceMS > 0 && directive.ThrottleMS > 0 {
		return EventDirective{}, fmt.Errorf("g:on:%s cannot combine debounce and throttle", raw)
	}
	return directive, nil
}

func parseEventDurationMS(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	multiplier := 1
	var number string
	switch {
	case strings.HasSuffix(value, "ms"):
		number = strings.TrimSuffix(value, "ms")
	case strings.HasSuffix(value, "s"):
		number = strings.TrimSuffix(value, "s")
		multiplier = 1000
	default:
		return 0, fmt.Errorf("duration must use ms or s")
	}
	raw, err := strconv.Atoi(strings.TrimSpace(number))
	if err != nil || raw <= 0 {
		return 0, fmt.Errorf("duration must be a positive integer")
	}
	return raw * multiplier, nil
}

// RuntimeOptions returns the compact modifier string emitted into HTML.
func (directive EventDirective) RuntimeOptions() string {
	var options []string
	if directive.Prevent {
		options = append(options, "prevent")
	}
	if directive.Stop {
		options = append(options, "stop")
	}
	if directive.Once {
		options = append(options, "once")
	}
	if directive.Capture {
		options = append(options, "capture")
	}
	if directive.DebounceMS > 0 {
		options = append(options, fmt.Sprintf("debounce:%d", directive.DebounceMS))
	}
	if directive.ThrottleMS > 0 {
		options = append(options, fmt.Sprintf("throttle:%d", directive.ThrottleMS))
	}
	return strings.Join(options, " ")
}
