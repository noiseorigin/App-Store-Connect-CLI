package reviews

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDeriveOutcome(t *testing.T) {
	tests := []struct {
		name            string
		submissionState string
		itemStates      []string
		want            string
	}{
		{
			name:            "all items approved",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED"},
			want:            "approved",
		},
		{
			name:            "any item rejected",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED", "REJECTED"},
			want:            "rejected",
		},
		{
			name:            "unresolved issues no rejected items",
			submissionState: "UNRESOLVED_ISSUES",
			itemStates:      []string{"ACCEPTED"},
			want:            "rejected",
		},
		{
			name:            "rejected item takes priority over unresolved",
			submissionState: "UNRESOLVED_ISSUES",
			itemStates:      []string{"REJECTED"},
			want:            "rejected",
		},
		{
			name:            "mixed non-rejected states falls through to submission state",
			submissionState: "COMPLETE",
			itemStates:      []string{"APPROVED", "ACCEPTED"},
			want:            "complete",
		},
		{
			name:            "no items uses submission state",
			submissionState: "WAITING_FOR_REVIEW",
			itemStates:      nil,
			want:            "waiting_for_review",
		},
		{
			name:            "in review state",
			submissionState: "IN_REVIEW",
			itemStates:      []string{"READY_FOR_REVIEW"},
			want:            "in_review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveOutcome(tt.submissionState, tt.itemStates)
			if got != tt.want {
				t.Errorf("deriveOutcome(%q, %v) = %q, want %q", tt.submissionState, tt.itemStates, got, tt.want)
			}
		})
	}
}

func TestSubmissionsHistoryCommand_MissingApp(t *testing.T) {
	cmd := SubmissionsHistoryCommand()
	if cmd.Name != "submissions-history" {
		t.Fatalf("unexpected command name: %s", cmd.Name)
	}

	// Unset any env that could provide app ID
	t.Setenv("ASC_APP_ID", "")

	err := cmd.ParseAndRun(context.Background(), []string{})
	if err == nil {
		t.Fatal("expected error for missing --app, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got: %v", err)
	}
}

func TestSubmissionsHistoryCommand_InvalidLimit(t *testing.T) {
	cmd := SubmissionsHistoryCommand()
	t.Setenv("ASC_APP_ID", "test-app")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")

	err := cmd.ParseAndRun(context.Background(), []string{"--limit", "999"})
	if err == nil {
		t.Fatal("expected error for invalid limit, got nil")
	}
	if !strings.Contains(err.Error(), "--limit must be between 1 and 200") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func testJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestHistoryClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(keyPath, data, 0o600); err != nil {
		t.Fatalf("write key error: %v", err)
	}

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient error: %v", err)
	}
	return client
}

func makeSubmissions(entries ...struct {
	id, platform, state, date string
},
) []asc.ReviewSubmissionResource {
	var subs []asc.ReviewSubmissionResource
	for _, e := range entries {
		subs = append(subs, asc.ReviewSubmissionResource{
			ID: e.id,
			Attributes: asc.ReviewSubmissionAttributes{
				Platform:        asc.Platform(e.platform),
				SubmissionState: asc.ReviewSubmissionState(e.state),
				SubmittedDate:   e.date,
			},
		})
	}
	return subs
}

func TestEnrichSubmissions_HappyPath(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-1"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/reviewSubmissions/sub-2/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-2",
					"attributes": {"state": "REJECTED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-2"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-2/items"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-1":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-1", "attributes": {"versionString": "3.1.1", "platform": "TV_OS"}},
				"links": {"self": "/v1/appStoreVersions/ver-1"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-2":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-2", "attributes": {"versionString": "3.0.0", "platform": "TV_OS"}},
				"links": {"self": "/v1/appStoreVersions/ver-2"}
			}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "TV_OS", "COMPLETE", "2026-03-01T12:00:00Z"},
		struct{ id, platform, state, date string }{"sub-2", "TV_OS", "UNRESOLVED_ISSUES", "2026-02-15T10:00:00Z"},
	)

	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Sorted by submittedDate descending
	if entries[0].VersionString != "3.1.1" {
		t.Errorf("first entry version = %q, want %q", entries[0].VersionString, "3.1.1")
	}
	if entries[0].Outcome != "approved" {
		t.Errorf("first entry outcome = %q, want %q", entries[0].Outcome, "approved")
	}
	if entries[1].VersionString != "3.0.0" {
		t.Errorf("second entry version = %q, want %q", entries[1].VersionString, "3.0.0")
	}
	if entries[1].Outcome != "rejected" {
		t.Errorf("second entry outcome = %q, want %q", entries[1].Outcome, "rejected")
	}
	if len(entries[0].Items) != 1 {
		t.Errorf("first entry items count = %d, want 1", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_EmptySubmissions(t *testing.T) {
	client := newTestHistoryClient(t, testRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Fatal("no API calls expected for empty submissions")
		return nil, nil
	}))
	entries, err := enrichSubmissions(context.Background(), client, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestEnrichSubmissions_Version404(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"},
					"relationships": {
						"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-gone"}}
					}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-gone":
			return testJSONResponse(404, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist"}]}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
}

func TestEnrichSubmissions_VersionFilter(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case path == "/v1/reviewSubmissions/sub-1/items":
			return testJSONResponse(200, `{
				"data": [{"type": "reviewSubmissionItems", "id": "item-1", "attributes": {"state": "APPROVED"},
					"relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-1"}}}}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		case path == "/v1/reviewSubmissions/sub-2/items":
			return testJSONResponse(200, `{
				"data": [{"type": "reviewSubmissionItems", "id": "item-2", "attributes": {"state": "APPROVED"},
					"relationships": {"appStoreVersion": {"data": {"type": "appStoreVersions", "id": "ver-2"}}}}],
				"links": {"self": "/v1/reviewSubmissions/sub-2/items"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-1":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-1", "attributes": {"versionString": "2.0.0", "platform": "IOS"}},
				"links": {"self": "/v1/appStoreVersions/ver-1"}
			}`), nil
		case path == "/v1/appStoreVersions/ver-2":
			return testJSONResponse(200, `{
				"data": {"type": "appStoreVersions", "id": "ver-2", "attributes": {"versionString": "1.0.0", "platform": "IOS"}},
				"links": {"self": "/v1/appStoreVersions/ver-2"}
			}`), nil
		default:
			return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
		}
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
		struct{ id, platform, state, date string }{"sub-2", "IOS", "COMPLETE", "2026-02-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after version filter, got %d", len(entries))
	}
	if entries[0].VersionString != "2.0.0" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "2.0.0")
	}
}

func TestEnrichSubmissions_NoItems(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(200, `{
				"data": [],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		}
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
	if entries[0].Outcome != "complete" {
		t.Errorf("outcome = %q, want %q", entries[0].Outcome, "complete")
	}
	if len(entries[0].Items) != 0 {
		t.Errorf("items count = %d, want 0", len(entries[0].Items))
	}
}

func TestEnrichSubmissions_ItemWithoutVersionRelationship(t *testing.T) {
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/reviewSubmissions/sub-1/items" {
			return testJSONResponse(200, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "APPROVED"}
				}],
				"links": {"self": "/v1/reviewSubmissions/sub-1/items"}
			}`), nil
		}
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-1", "IOS", "COMPLETE", "2026-03-01T12:00:00Z"},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].VersionString != "unknown" {
		t.Errorf("version = %q, want %q", entries[0].VersionString, "unknown")
	}
	if len(entries[0].Items) != 1 {
		t.Errorf("items count = %d, want 1", len(entries[0].Items))
	}
	if entries[0].Items[0].ResourceID != "" {
		t.Errorf("item resourceId = %q, want empty", entries[0].Items[0].ResourceID)
	}
}

func TestPrintHistoryTable_NoError(t *testing.T) {
	entries := []SubmissionHistoryEntry{
		{
			SubmissionID:  "sub-1",
			VersionString: "3.1.1",
			Platform:      "TV_OS",
			State:         "COMPLETE",
			SubmittedDate: "2026-03-01T12:00:00Z",
			Outcome:       "approved",
			Items:         []SubmissionHistoryItem{{ID: "i1", State: "APPROVED", Type: "appStoreVersion", ResourceID: "v1"}},
		},
	}
	err := printHistoryTable(entries)
	if err != nil {
		t.Fatalf("printHistoryTable error: %v", err)
	}
}

func TestFormatItemsSummary(t *testing.T) {
	tests := []struct {
		name  string
		items []SubmissionHistoryItem
		want  string
	}{
		{"no items", nil, "0 items"},
		{"single approved", []SubmissionHistoryItem{{State: "APPROVED"}}, "1 approved"},
		{"mixed", []SubmissionHistoryItem{{State: "APPROVED"}, {State: "REJECTED"}}, "1 approved, 1 rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatItemsSummary(tt.items)
			if got != tt.want {
				t.Errorf("formatItemsSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnrichSubmissions_SkipsEmptySubmittedDate(t *testing.T) {
	calls := 0
	transport := testRoundTripper(func(req *http.Request) (*http.Response, error) {
		calls++
		return testJSONResponse(404, `{"errors":[{"status":"404"}]}`), nil
	})

	subs := makeSubmissions(
		struct{ id, platform, state, date string }{"sub-draft", "IOS", "READY_FOR_REVIEW", ""},
	)
	client := newTestHistoryClient(t, transport)
	entries, err := enrichSubmissions(context.Background(), client, subs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (draft skipped), got %d", len(entries))
	}
	if calls != 0 {
		t.Errorf("expected 0 API calls for draft submissions, got %d", calls)
	}
}
