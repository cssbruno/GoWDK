package gointerop

import (
	"log/slog"
	"net/mail"
	"os"
	"sort"
	"strings"
)

// SubscriberDigest is build-time data produced by delegating real work to the
// standard library: addresses are parsed and validated with net/mail and the
// build emits structured logs through log/slog. It demonstrates a .gwdk page
// handing serious behavior to a normal Go package instead of inline or
// generated business logic.
type SubscriberDigest struct {
	Title         string `json:"title"`
	Tagline       string `json:"tagline"`
	ValidCount    int    `json:"validCount"`
	RejectedCount int    `json:"rejectedCount"`
	SampleDomains string `json:"sampleDomains"`
}

// rawSubscribers stands in for a real data source — a database/sql or pgx query,
// a CRM export, or a drained queue. It is intentionally hardcoded so the example
// adds no production dependency: swap this slice for your data layer and the
// .gwdk build contract is unchanged.
var rawSubscribers = []string{
	"Ada Lovelace <ada@example.com>",
	"grace@example.org",
	"not-an-email",
	"Alan Turing <alan@example.net>",
	"   ",
}

// SubscriberDigestForBuild validates rawSubscribers with net/mail, logs the
// outcome with log/slog (to stderr, which GOWDK keeps separate from the JSON
// build payload), and returns the digest rendered by the page.
func SubscriberDigestForBuild() SubscriberDigest {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	valid, rejected := 0, 0
	domains := map[string]struct{}{}
	for _, raw := range rawSubscribers {
		trimmed := strings.TrimSpace(raw)
		address, err := mail.ParseAddress(trimmed)
		if err != nil {
			rejected++
			log.Warn("rejected subscriber", "raw", raw, "error", err.Error())
			continue
		}
		valid++
		if at := strings.LastIndex(address.Address, "@"); at >= 0 {
			domains[strings.ToLower(address.Address[at+1:])] = struct{}{}
		}
	}

	sorted := make([]string, 0, len(domains))
	for domain := range domains {
		sorted = append(sorted, domain)
	}
	sort.Strings(sorted)

	log.Info("subscriber digest built", "valid", valid, "rejected", rejected, "domains", len(sorted))

	return SubscriberDigest{
		Title:         "Newsletter digest",
		Tagline:       "Validated at build time with net/mail; structured logs via log/slog.",
		ValidCount:    valid,
		RejectedCount: rejected,
		SampleDomains: strings.Join(sorted, ", "),
	}
}
