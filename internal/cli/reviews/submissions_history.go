package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubmissionHistoryEntry is the assembled result for one submission.
type SubmissionHistoryEntry struct {
	SubmissionID  string                  `json:"submissionId"`
	VersionString string                  `json:"versionString"`
	Platform      string                  `json:"platform"`
	State         string                  `json:"state"`
	SubmittedDate string                  `json:"submittedDate"`
	Outcome       string                  `json:"outcome"`
	Items         []SubmissionHistoryItem `json:"items"`
}

// SubmissionHistoryItem is a summary of one item in a submission.
type SubmissionHistoryItem struct {
	ID         string `json:"id"`
	State      string `json:"state"`
	Type       string `json:"type"`
	ResourceID string `json:"resourceId"`
}

// SubmissionsHistoryCommand returns the submissions-history subcommand.
func SubmissionsHistoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-history", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by state (comma-separated)")
	version := fs.String("version", "", "Filter by version string (e.g. 1.2.0)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-history",
		ShortUsage: "asc review submissions-history [flags]",
		ShortHelp:  "Show enriched review submission history for an app.",
		LongHelp: `Show enriched review submission history for an app.

Each entry includes the submission state, platform, version string, submitted
date, and a derived outcome (approved, rejected, or the raw state).

Examples:
  asc review submissions-history --app "123456789"
  asc review submissions-history --app "123456789" --platform IOS --state COMPLETE
  asc review submissions-history --app "123456789" --version "1.2.0"
  asc review submissions-history --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review submissions-history: --limit must be between 1 and 200")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}
			states := shared.SplitCSVUpper(*state)

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionsOption{
				asc.WithReviewSubmissionsLimit(*limit),
				asc.WithReviewSubmissionsPlatforms(platforms),
				asc.WithReviewSubmissionsStates(states),
			}

			// Fetch submissions (with or without pagination)
			var submissions []asc.ReviewSubmissionResource
			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionsLimit(200))
				resp, pErr := shared.PaginateWithSpinner(requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissions(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissions(ctx, resolvedAppID, asc.WithReviewSubmissionsNextURL(nextURL))
					},
				)
				if pErr != nil {
					return fmt.Errorf("review submissions-history: %w", pErr)
				}
				if aggResp, ok := resp.(*asc.ReviewSubmissionsResponse); ok {
					submissions = aggResp.Data
				}
			} else {
				resp, fErr := client.GetReviewSubmissions(requestCtx, resolvedAppID, opts...)
				if fErr != nil {
					return fmt.Errorf("review submissions-history: %w", fErr)
				}
				submissions = resp.Data
			}

			// Enrich with items + version strings
			entries, err := enrichSubmissions(requestCtx, client, submissions, strings.TrimSpace(*version))
			if err != nil {
				return fmt.Errorf("review submissions-history: %w", err)
			}

			tableFunc := func() error { return printHistoryTable(entries) }
			markdownFunc := func() error { return printHistoryMarkdown(entries) }
			return shared.PrintOutputWithRenderers(entries, *output.Output, *output.Pretty, tableFunc, markdownFunc)
		},
	}
}

// enrichSubmissions takes already-fetched submissions and enriches each with
// item states and version strings. Applies client-side version filtering and
// sorts by submittedDate descending.
func enrichSubmissions(ctx context.Context, client *asc.Client, submissions []asc.ReviewSubmissionResource, versionFilter string) ([]SubmissionHistoryEntry, error) {
	var entries []SubmissionHistoryEntry
	for _, sub := range submissions {
		// Skip pre-submission drafts (no submittedDate)
		if strings.TrimSpace(sub.Attributes.SubmittedDate) == "" {
			continue
		}

		entry := SubmissionHistoryEntry{
			SubmissionID:  sub.ID,
			Platform:      string(sub.Attributes.Platform),
			State:         string(sub.Attributes.SubmissionState),
			SubmittedDate: sub.Attributes.SubmittedDate,
		}

		// Fetch items for this submission
		itemsResp, err := client.GetReviewSubmissionItems(ctx, sub.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch items for submission %s: %w", sub.ID, err)
		}

		var itemStates []string
		for _, item := range itemsResp.Data {
			histItem := SubmissionHistoryItem{
				ID:    item.ID,
				State: item.Attributes.State,
			}

			// Extract version relationship if present
			if item.Relationships != nil && item.Relationships.AppStoreVersion != nil {
				histItem.Type = "appStoreVersion"
				histItem.ResourceID = item.Relationships.AppStoreVersion.Data.ID

				// Fetch version string
				if histItem.ResourceID != "" {
					verResp, verErr := client.GetAppStoreVersion(ctx, histItem.ResourceID)
					if verErr != nil {
						if asc.IsNotFound(verErr) {
							entry.VersionString = "unknown"
						} else {
							return nil, fmt.Errorf("failed to fetch version %s: %w", histItem.ResourceID, verErr)
						}
					} else if entry.VersionString == "" {
						entry.VersionString = verResp.Data.Attributes.VersionString
					}
				}
			}

			itemStates = append(itemStates, item.Attributes.State)
			entry.Items = append(entry.Items, histItem)
		}

		entry.Outcome = deriveOutcome(entry.State, itemStates)

		if entry.VersionString == "" {
			entry.VersionString = "unknown"
		}

		entries = append(entries, entry)
	}

	// Client-side version filter
	if versionFilter != "" {
		var filtered []SubmissionHistoryEntry
		for _, e := range entries {
			if e.VersionString == versionFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by submittedDate descending
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SubmittedDate > entries[j].SubmittedDate
	})

	return entries, nil
}

func printHistoryTable(entries []SubmissionHistoryEntry) error {
	headers := []string{"VERSION", "PLATFORM", "STATE", "SUBMITTED", "OUTCOME", "ITEMS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.VersionString,
			e.Platform,
			e.State,
			e.SubmittedDate,
			e.Outcome,
			formatItemsSummary(e.Items),
		})
	}
	asc.RenderTable(headers, rows)
	return nil
}

func printHistoryMarkdown(entries []SubmissionHistoryEntry) error {
	headers := []string{"VERSION", "PLATFORM", "STATE", "SUBMITTED", "OUTCOME", "ITEMS"}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{
			e.VersionString,
			e.Platform,
			e.State,
			e.SubmittedDate,
			e.Outcome,
			formatItemsSummary(e.Items),
		})
	}
	asc.RenderMarkdown(headers, rows)
	return nil
}

func formatItemsSummary(items []SubmissionHistoryItem) string {
	if len(items) == 0 {
		return "0 items"
	}
	counts := map[string]int{}
	for _, item := range items {
		counts[strings.ToLower(item.State)]++
	}
	var parts []string
	for state, count := range counts {
		parts = append(parts, fmt.Sprintf("%d %s", count, state))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// deriveOutcome computes a human-readable outcome from submission and item states.
// Priority order:
// 1. Any item REJECTED → "rejected"
// 2. All items APPROVED → "approved"
// 3. Submission state UNRESOLVED_ISSUES → "rejected"
// 4. Fallback → lowercase submission state
func deriveOutcome(submissionState string, itemStates []string) string {
	hasRejected := false
	allApproved := len(itemStates) > 0

	for _, s := range itemStates {
		if s == "REJECTED" {
			hasRejected = true
		}
		if s != "APPROVED" {
			allApproved = false
		}
	}

	if hasRejected {
		return "rejected"
	}
	if allApproved {
		return "approved"
	}
	if submissionState == "UNRESOLVED_ISSUES" {
		return "rejected"
	}
	return strings.ToLower(submissionState)
}
