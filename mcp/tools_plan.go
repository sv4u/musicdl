package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	dlplan "github.com/sv4u/musicdl/download/plan"
)

func registerPlanTools(server *mcp.Server, provider DataProvider) {
	type getPlanParams struct {
		Status string `json:"status,omitempty" jsonschema:"Optional: pending, in_progress, completed, failed, or skipped"`
		Source string `json:"source,omitempty" jsonschema:"Optional: spotify, youtube, soundcloud, bandcamp, or audius"`
		Limit  int    `json:"limit,omitempty" jsonschema:"Max plan items to list (default 50)"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_plan",
		Description: "Load the current download plan with aggregate statistics and a filtered list of plan items.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in getPlanParams) (*mcp.CallToolResult, any, error) {
		p, hash, err := provider.GetCurrentPlan()
		if err != nil {
			return nil, nil, err
		}
		if p == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No plan is loaded."}}}, nil, nil
		}
		items := p.Items
		if items == nil {
			items = []*dlplan.PlanItem{}
		}
		wantStatus, hasStatus, err := parseStatusFilter(in.Status)
		if err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}, IsError: true}, nil, nil
		}
		wantSource, hasSource, err := parseSourceFilter(in.Source)
		if err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}, IsError: true}, nil, nil
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 2000 {
			limit = 2000
		}
		filtered := filterPlanItems(items, hasStatus, wantStatus, hasSource, wantSource)
		stats := p.GetStatistics()
		exec := p.GetExecutionStatistics()
		byStatus := countByStatus(items)
		bySource := countBySource(items)
		var b strings.Builder
		b.WriteString("# Current download plan\n\n")
		b.WriteString("## Overview\n")
		b.WriteString(fmt.Sprintf("- **Plan hash**: `%s`\n", hash))
		if ti, ok := stats["total_items"].(int); ok {
			b.WriteString(fmt.Sprintf("- **Total items**: %d\n", ti))
		} else {
			b.WriteString(fmt.Sprintf("- **Total items**: %d\n", len(items)))
		}
		b.WriteString("\n## By type\n")
		if bt, ok := stats["by_type"].(map[string]int); ok {
			keys := sortedStringKeysFromIntMap(bt)
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("- **%s**: %d\n", k, bt[k]))
			}
		}
		b.WriteString("\n## By status (all items)\n")
		for _, k := range sortedStringKeysFromStatusMap(byStatus) {
			b.WriteString(fmt.Sprintf("- **%s**: %d\n", k, byStatus[dlplan.PlanItemStatus(k)]))
		}
		b.WriteString("\n## By source\n")
		for _, k := range sortedStrings(bySource) {
			b.WriteString(fmt.Sprintf("- **%s**: %d\n", k, bySource[k]))
		}
		b.WriteString("\n## Track execution (track items only)\n")
		b.WriteString(fmt.Sprintf("- **Completed**: %d\n", exec["completed"]))
		b.WriteString(fmt.Sprintf("- **Failed**: %d\n", exec["failed"]))
		b.WriteString(fmt.Sprintf("- **Pending**: %d\n", exec["pending"]))
		b.WriteString(fmt.Sprintf("- **In progress**: %d\n", exec["in_progress"]))
		b.WriteString(fmt.Sprintf("- **Skipped**: %d\n", exec["skipped"]))
		b.WriteString(fmt.Sprintf("- **Total (tracks)**: %d\n", exec["total"]))
		b.WriteString("\n## Listed items\n")
		b.WriteString(fmt.Sprintf("_Showing up to **%d** of **%d** items after filters._\n\n", min(len(filtered), limit), len(filtered)))
		shown := 0
		for _, it := range filtered {
			if shown >= limit {
				break
			}
			formatPlanItem(&b, it)
			b.WriteString("\n")
			shown++
		}
		if len(filtered) == 0 {
			b.WriteString("_No items match the filters._\n")
		} else if len(filtered) > shown {
			b.WriteString(fmt.Sprintf("\n_… %d more not shown (increase `limit` or narrow filters)._\n", len(filtered)-shown))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})

	type searchPlanParams struct {
		Query  string `json:"query" jsonschema:"Search text (matches name, artist/album metadata, Spotify URL, source URL)"`
		Status string `json:"status,omitempty" jsonschema:"Optional status filter"`
		Limit  int    `json:"limit,omitempty" jsonschema:"Max matches to return (default 30)"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_plan",
		Description: "Search plan items by name, artist/album metadata, Spotify URL, or source URL.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in searchPlanParams) (*mcp.CallToolResult, any, error) {
		q := strings.TrimSpace(in.Query)
		if q == "" {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "`query` is required."}}, IsError: true}, nil, nil
		}
		p, _, err := provider.GetCurrentPlan()
		if err != nil {
			return nil, nil, err
		}
		if p == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No plan is loaded."}}}, nil, nil
		}
		items := p.Items
		if items == nil {
			items = []*dlplan.PlanItem{}
		}
		wantStatus, hasStatus, err := parseStatusFilter(in.Status)
		if err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}, IsError: true}, nil, nil
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 30
		}
		if limit > 2000 {
			limit = 2000
		}
		qLower := strings.ToLower(q)
		var matches []*dlplan.PlanItem
		for _, it := range items {
			if hasStatus && it.GetStatus() != wantStatus {
				continue
			}
			if planItemMatchesQuery(it, qLower) {
				matches = append(matches, it)
			}
		}
		var b strings.Builder
		b.WriteString("# Plan search results\n\n")
		b.WriteString(fmt.Sprintf("_Query_: **%s** — **%d** match(es)\n\n", q, len(matches)))
		shown := 0
		for _, it := range matches {
			if shown >= limit {
				break
			}
			formatPlanItem(&b, it)
			b.WriteString("\n")
			shown++
		}
		if len(matches) == 0 {
			b.WriteString("_No matching items._\n")
		} else if len(matches) > shown {
			b.WriteString(fmt.Sprintf("\n_… %d more not shown._\n", len(matches)-shown))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_plan_files",
		Description: "List saved plan files on disk with hash, modification time, size, and track counts.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		files, err := provider.ListPlanFiles()
		if err != nil {
			return nil, nil, err
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModifiedAt.After(files[j].ModifiedAt)
		})
		var b strings.Builder
		b.WriteString("# Saved plan files\n\n")
		if len(files) == 0 {
			b.WriteString("_No plan files found._\n")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: b.String()}}}, nil, nil
		}
		b.WriteString(fmt.Sprintf("**%d** file(s)\n\n", len(files)))
		for i := range files {
			f := &files[i]
			b.WriteString(fmt.Sprintf("## %d. `%s`\n", i+1, f.Hash))
			b.WriteString(fmt.Sprintf("- **Path**: `%s`\n", f.Path))
			b.WriteString(fmt.Sprintf("- **Modified**: %s\n", f.ModifiedAt.UTC().Format(time.RFC3339)))
			b.WriteString(fmt.Sprintf("- **Size**: %s\n", formatHumanBytes(f.SizeBytes)))
			b.WriteString(fmt.Sprintf("- **Track count**: %d\n", f.TrackCount))
			b.WriteString("\n")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})
}

func parseStatusFilter(s string) (dlplan.PlanItemStatus, bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", false, nil
	}
	switch s {
	case string(dlplan.PlanItemStatusPending),
		string(dlplan.PlanItemStatusInProgress),
		string(dlplan.PlanItemStatusCompleted),
		string(dlplan.PlanItemStatusFailed),
		string(dlplan.PlanItemStatusSkipped):
		return dlplan.PlanItemStatus(s), true, nil
	default:
		return "", false, fmt.Errorf("invalid status %q: use pending, in_progress, completed, failed, or skipped", s)
	}
}

func parseSourceFilter(s string) (dlplan.SourceType, bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", false, nil
	}
	switch s {
	case string(dlplan.SourceTypeSpotify),
		string(dlplan.SourceTypeYouTube),
		string(dlplan.SourceTypeSoundCloud),
		string(dlplan.SourceTypeBandcamp),
		string(dlplan.SourceTypeAudius):
		return dlplan.SourceType(s), true, nil
	default:
		return "", false, fmt.Errorf("invalid source %q: use spotify, youtube, soundcloud, bandcamp, or audius", s)
	}
}

func effectiveSource(it *dlplan.PlanItem) string {
	if it.Source != "" {
		return string(it.Source)
	}
	if it.SpotifyURL != "" {
		return string(dlplan.SourceTypeSpotify)
	}
	if it.YouTubeURL != "" {
		return string(dlplan.SourceTypeYouTube)
	}
	return "unknown"
}

func filterPlanItems(items []*dlplan.PlanItem, hasStatus bool, wantStatus dlplan.PlanItemStatus, hasSource bool, wantSource dlplan.SourceType) []*dlplan.PlanItem {
	var out []*dlplan.PlanItem
	for _, it := range items {
		if hasStatus && it.GetStatus() != wantStatus {
			continue
		}
		if hasSource {
			es := effectiveSource(it)
			if !strings.EqualFold(es, string(wantSource)) {
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

func countByStatus(items []*dlplan.PlanItem) map[dlplan.PlanItemStatus]int {
	m := map[dlplan.PlanItemStatus]int{}
	for _, it := range items {
		m[it.GetStatus()]++
	}
	return m
}

func countBySource(items []*dlplan.PlanItem) map[string]int {
	m := map[string]int{}
	for _, it := range items {
		k := effectiveSource(it)
		m[k]++
	}
	return m
}

func sortedStringKeysFromIntMap(bt map[string]int) []string {
	var keys []string
	for k := range bt {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedStringKeysFromStatusMap(bt map[dlplan.PlanItemStatus]int) []string {
	var keys []string
	for k := range bt {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	return keys
}

func sortedStrings(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func planItemMatchesQuery(it *dlplan.PlanItem, qLower string) bool {
	if strings.Contains(strings.ToLower(it.Name), qLower) {
		return true
	}
	if it.SpotifyURL != "" && strings.Contains(strings.ToLower(it.SpotifyURL), qLower) {
		return true
	}
	if it.SourceURL != "" && strings.Contains(strings.ToLower(it.SourceURL), qLower) {
		return true
	}
	md := it.GetMetadata()
	for _, key := range []string{"artist", "album", "Artist", "Album"} {
		v, ok := md[key]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(s), qLower) {
			return true
		}
	}
	return false
}

func formatPlanItem(b *strings.Builder, it *dlplan.PlanItem) {
	st, started, completed := it.GetTimestamps()
	b.WriteString(fmt.Sprintf("### %s\n", it.Name))
	b.WriteString(fmt.Sprintf("- **ID**: `%s`\n", it.ItemID))
	b.WriteString(fmt.Sprintf("- **Type**: %s\n", it.ItemType))
	b.WriteString(fmt.Sprintf("- **Status**: %s\n", it.GetStatus()))
	if it.Source != "" {
		b.WriteString(fmt.Sprintf("- **Source**: %s\n", it.Source))
	}
	if it.SpotifyURL != "" {
		b.WriteString(fmt.Sprintf("- **Spotify URL**: %s\n", it.SpotifyURL))
	}
	if it.YouTubeURL != "" {
		b.WriteString(fmt.Sprintf("- **YouTube URL**: %s\n", it.YouTubeURL))
	}
	if it.SourceURL != "" {
		b.WriteString(fmt.Sprintf("- **Source URL**: %s\n", it.SourceURL))
	}
	if it.ParentID != "" {
		b.WriteString(fmt.Sprintf("- **Parent ID**: `%s`\n", it.ParentID))
	}
	b.WriteString(fmt.Sprintf("- **Progress**: %.1f%%\n", it.GetProgress()*100))
	if fp := it.GetFilePath(); fp != "" {
		b.WriteString(fmt.Sprintf("- **File**: `%s`\n", fp))
	}
	if err := it.GetError(); err != "" {
		b.WriteString(fmt.Sprintf("- **Error**: %s\n", err))
	}
	b.WriteString(fmt.Sprintf("- **Created**: %s\n", st.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **Started**: %s\n", formatTimePtr(started)))
	b.WriteString(fmt.Sprintf("- **Completed**: %s\n", formatTimePtr(completed)))
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.UTC().Format(time.RFC3339)
}
