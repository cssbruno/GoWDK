package clientlang

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// This file holds the deterministic formatting and date/time builtins shared by
// the Go evaluator (build-time/SSR) and mirrored byte-for-byte by the browser
// runtime in internal/clientrt/assets/island.js. Every operation is implemented
// with plain IEEE-754 doubles and integer arithmetic so Go and JavaScript
// produce identical output — no locale tables, no Intl/Date, no rounding-mode
// divergence. The expression conformance test cross-checks both evaluators.

// maxFormatDigits bounds the fractional precision of the formatting builtins so
// the decimal scale stays an exactly representable power of ten.
const maxFormatDigits = 20

// formatScale returns 10^digits built by repeated multiplication so the value is
// identical in Go and JavaScript (math.Pow / Math.pow are not guaranteed
// bit-identical across runtimes).
func formatScale(digits int) float64 {
	scale := 1.0
	for index := 0; index < digits; index++ {
		scale *= 10
	}
	return scale
}

// roundHalfAway rounds to the nearest integer with ties going away from zero,
// using only IEEE-754 floor/abs so Go (math.Floor) and JavaScript (Math.floor)
// agree on every input.
func roundHalfAway(value float64) float64 {
	if value < 0 {
		return -math.Floor(-value + 0.5)
	}
	return math.Floor(value + 0.5)
}

// formatFixed renders value with exactly digits fractional places.
func formatFixed(value float64, digits int) (string, error) {
	if digits < 0 || digits > maxFormatDigits {
		return "", fmt.Errorf("built-in fixed expects digits in [0, %d]", maxFormatDigits)
	}
	if !isFiniteFloat(value) {
		return "", fmt.Errorf("built-in fixed expects a finite number")
	}
	scale := formatScale(digits)
	negative := value < 0
	scaled := roundHalfAway(math.Abs(value) * scale)
	rawDigits := strconv.FormatFloat(scaled, 'f', 0, 64)
	var out string
	if digits == 0 {
		out = rawDigits
	} else {
		for len(rawDigits) <= digits {
			rawDigits = "0" + rawDigits
		}
		split := len(rawDigits) - digits
		out = rawDigits[:split] + "." + rawDigits[split:]
	}
	if negative && !isAllZeroDigits(out) {
		out = "-" + out
	}
	return out, nil
}

// roundTo rounds value to digits fractional places and returns the number.
func roundTo(value float64, digits int) (float64, error) {
	if digits < 0 || digits > maxFormatDigits {
		return 0, fmt.Errorf("built-in round expects digits in [0, %d]", maxFormatDigits)
	}
	if !isFiniteFloat(value) {
		return 0, fmt.Errorf("built-in round expects a finite number")
	}
	scale := formatScale(digits)
	return roundHalfAway(value*scale) / scale, nil
}

// formatPercent renders value*100 with digits fractional places and a percent
// sign.
func formatPercent(value float64, digits int) (string, error) {
	formatted, err := formatFixed(value*100, digits)
	if err != nil {
		return "", err
	}
	return formatted + "%", nil
}

// formatUnixTime renders a UTC timestamp (whole seconds since the Unix epoch)
// using a small token layout: YYYY, MM, DD, HH, mm, ss. Any other character is
// copied literally. Output is always UTC so it never depends on the host time
// zone.
func formatUnixTime(unix float64, layout string) (string, error) {
	if !isFiniteFloat(unix) || unix != math.Floor(unix) {
		return "", fmt.Errorf("built-in formatTime expects an integer unix timestamp")
	}
	seconds := int64(unix)
	days := floorDivInt(seconds, 86400)
	secondOfDay := seconds - days*86400
	hour := secondOfDay / 3600
	minute := (secondOfDay / 60) % 60
	second := secondOfDay % 60
	year, month, day := civilFromDays(days)
	return expandTimeLayout(layout, year, month, day, hour, minute, second), nil
}

// civilFromDays converts a count of days since the Unix epoch into a UTC
// year/month/day using Howard Hinnant's days_from_civil inverse, which is exact
// integer arithmetic valid across the whole representable range.
func civilFromDays(days int64) (year int64, month int64, day int64) {
	z := days + 719468
	era := floorDivInt(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	day = doy - (153*mp+2)/5 + 1
	if mp < 10 {
		month = mp + 3
	} else {
		month = mp - 9
	}
	if month <= 2 {
		y++
	}
	return y, month, day
}

func expandTimeLayout(layout string, year, month, day, hour, minute, second int64) string {
	var builder strings.Builder
	for index := 0; index < len(layout); {
		switch {
		case strings.HasPrefix(layout[index:], "YYYY"):
			builder.WriteString(padNumber(year, 4))
			index += 4
		case strings.HasPrefix(layout[index:], "MM"):
			builder.WriteString(padNumber(month, 2))
			index += 2
		case strings.HasPrefix(layout[index:], "DD"):
			builder.WriteString(padNumber(day, 2))
			index += 2
		case strings.HasPrefix(layout[index:], "HH"):
			builder.WriteString(padNumber(hour, 2))
			index += 2
		case strings.HasPrefix(layout[index:], "mm"):
			builder.WriteString(padNumber(minute, 2))
			index += 2
		case strings.HasPrefix(layout[index:], "ss"):
			builder.WriteString(padNumber(second, 2))
			index += 2
		default:
			builder.WriteByte(layout[index])
			index++
		}
	}
	return builder.String()
}

// padNumber renders value zero-padded to at least width digits. A negative value
// keeps its sign ahead of the padded magnitude.
func padNumber(value int64, width int) string {
	negative := value < 0
	magnitude := value
	if negative {
		magnitude = -value
	}
	digits := strconv.FormatInt(magnitude, 10)
	for len(digits) < width {
		digits = "0" + digits
	}
	if negative {
		return "-" + digits
	}
	return digits
}

// floorDivInt is integer division that rounds toward negative infinity, matching
// Math.floor(a / b) in JavaScript for the civil-date math.
func floorDivInt(numerator, denominator int64) int64 {
	quotient := numerator / denominator
	if (numerator%denominator != 0) && ((numerator < 0) != (denominator < 0)) {
		quotient--
	}
	return quotient
}

func isFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func isAllZeroDigits(value string) bool {
	for _, char := range value {
		if char != '0' && char != '.' {
			return false
		}
	}
	return true
}
