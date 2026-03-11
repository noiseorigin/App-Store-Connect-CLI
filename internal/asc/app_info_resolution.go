package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type appStoreVersionRelationships struct {
	App *Relationship `json:"app"`
}

type appInfoCandidate struct {
	id    string
	state string
}

// ResolveAppInfoIDForAppStoreVersion resolves the app info backing a version-scoped workflow.
func (c *Client) ResolveAppInfoIDForAppStoreVersion(ctx context.Context, versionID string) (string, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return "", fmt.Errorf("versionID is required")
	}

	versionResp, err := c.GetAppStoreVersion(ctx, versionID, WithAppStoreVersionInclude([]string{"app"}))
	if err != nil {
		return "", err
	}

	var relationships appStoreVersionRelationships
	if len(versionResp.Data.Relationships) > 0 {
		if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
			return "", fmt.Errorf("failed to parse app store version relationships: %w", err)
		}
	}

	appID := ""
	if relationships.App != nil {
		appID = strings.TrimSpace(relationships.App.Data.ID)
	}
	if appID == "" {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionID)
	}

	appInfos, err := c.GetAppInfos(ctx, appID)
	if err != nil {
		return "", err
	}
	if len(appInfos.Data) == 0 {
		return "", fmt.Errorf("no app info found for app %q", appID)
	}
	if len(appInfos.Data) == 1 {
		return strings.TrimSpace(appInfos.Data[0].ID), nil
	}

	candidates := make([]appInfoCandidate, 0, len(appInfos.Data))
	for _, item := range appInfos.Data {
		candidates = append(candidates, appInfoCandidate{
			id:    strings.TrimSpace(item.ID),
			state: appInfoState(item.Attributes),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].id < candidates[j].id
	})

	if resolvedID, ok := autoResolveAppInfoIDByVersionState(candidates, resolveAppStoreVersionState(versionResp.Data.Attributes)); ok {
		return resolvedID, nil
	}

	return "", fmt.Errorf(
		"multiple app infos found for app %q (%s); run `asc apps info list --app %q` to inspect candidates and use the app-info based age-rating flow explicitly",
		appID,
		formatAppInfoCandidates(candidates),
		appID,
	)
}

func resolveAppStoreVersionState(attrs AppStoreVersionAttributes) string {
	if trimmed := strings.TrimSpace(attrs.AppVersionState); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(attrs.AppStoreState)
}

func autoResolveAppInfoIDByVersionState(candidates []appInfoCandidate, versionState string) (string, bool) {
	resolvedVersionState := strings.TrimSpace(versionState)
	if resolvedVersionState == "" {
		return "", false
	}

	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.id == "" || !strings.EqualFold(candidate.state, resolvedVersionState) {
			continue
		}
		matches = append(matches, candidate.id)
	}
	if len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}

func appInfoState(attributes AppInfoAttributes) string {
	for _, key := range []string{"state", "appStoreState"} {
		rawValue, exists := attributes[key]
		if !exists || rawValue == nil {
			continue
		}
		value, ok := rawValue.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func formatAppInfoCandidates(candidates []appInfoCandidate) string {
	if len(candidates) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		state := candidate.state
		if state == "" {
			state = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s[state=%s]", candidate.id, state))
	}
	return strings.Join(parts, ", ")
}
