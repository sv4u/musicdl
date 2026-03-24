package audio

import (
	"context"
	"log"
	"math"
	"regexp"
	"strings"
)

var featPattern = regexp.MustCompile(`\s*[\(\[](feat\.?|ft\.?|featuring)[^\)\]]*[\)\]]`)
var specialCharsPattern = regexp.MustCompile(`[&+!@#$%^*(){}|\\<>]`)

// livePattern matches whole-word occurrences of keywords indicating a live/non-studio recording.
var livePattern = regexp.MustCompile(`(?i)\b(live|concert|performance|acoustic|unplugged|session|festival|tour|bootleg)\b|ライブ`)

// SearchCriteria carries expected metadata so the search can validate and rank results.
type SearchCriteria struct {
	ExpectedTitle      string
	ExpectedArtist     string
	ExpectedDurationS  int
	OriginalHasLiveTag bool // true when the Spotify title already contains "live"/"concert"/etc.
}

// GenerateSearchVariants produces multiple query variants for a given artist+title pair.
// Variants are ordered from most specific to most relaxed.
func GenerateSearchVariants(artist, title string) []string {
	seen := make(map[string]bool)
	var variants []string
	add := func(q string) {
		q = strings.TrimSpace(q)
		normalized := strings.ToLower(q)
		if q == "" || seen[normalized] {
			return
		}
		seen[normalized] = true
		variants = append(variants, q)
	}

	add(artist + " - " + title)

	strippedTitle := featPattern.ReplaceAllString(title, "")
	strippedTitle = strings.TrimSpace(strippedTitle)
	if strippedTitle != title {
		add(artist + " - " + strippedTitle)
	}

	cleanTitle := specialCharsPattern.ReplaceAllString(title, " ")
	cleanTitle = collapseSpaces(cleanTitle)
	if cleanTitle != title {
		add(artist + " - " + cleanTitle)
	}

	add(title)
	if strippedTitle != title {
		add(strippedTitle)
	}

	cleanArtist := specialCharsPattern.ReplaceAllString(artist, " ")
	cleanArtist = collapseSpaces(cleanArtist)
	add(cleanArtist + " " + cleanTitle)

	return variants
}

func collapseSpaces(s string) string {
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

// SearchWithFallbacks tries query variants sequentially (most specific first) across
// all providers. When SearchCriteria is provided, multiple yt-dlp results are fetched
// per query and scored so that live/concert versions are deprioritised for studio tracks.
func (p *Provider) SearchWithFallbacks(ctx context.Context, variants []string, criteria *SearchCriteria) (string, error) {
	if len(variants) == 0 {
		return "", &SearchError{Message: "no search variants provided"}
	}

	var lastErr error
	for _, query := range variants {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		cacheKey := p.normalizeQuery(query)
		if cached := p.searchCache.Get(cacheKey); cached != nil {
			if url, ok := cached.(string); ok && url != "" {
				return url, nil
			}
		}

		url, err := p.searchWithCriteria(ctx, query, criteria)
		if err == nil && url != "" {
			for _, v := range variants {
				p.searchCache.Set(p.normalizeQuery(v), url)
			}
			return url, nil
		}
		if err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", &SearchError{Message: "No audio found across all search variants"}
}

// searchWithCriteria searches all configured providers for a single query,
// applying criteria-based scoring when criteria is non-nil.
func (p *Provider) searchWithCriteria(ctx context.Context, query string, criteria *SearchCriteria) (string, error) {
	for _, provider := range p.config.AudioProviders {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if limiter, ok := p.rateLimiters[provider]; ok {
			if err := limiter.WaitIfNeeded(ctx); err != nil {
				continue
			}
		}
		url, err := p.searchProviderWithCriteria(ctx, provider, query, criteria)
		if err == nil && url != "" {
			return url, nil
		}
	}
	return "", &SearchError{Message: "No audio found for: " + query}
}

// TitleContainsLiveKeyword returns true if the title contains any live-performance keyword
// as a whole word (e.g. "Live" matches but "Alive" or "Oliver" do not).
func TitleContainsLiveKeyword(title string) bool {
	return livePattern.MatchString(title)
}

// scoreResult scores a yt-dlp search result against the expected criteria.
// Lower score is better. Returns math.MaxFloat64 if the result should be rejected.
func scoreResult(result ytDlpSearchResult, criteria *SearchCriteria) float64 {
	if criteria == nil {
		return 0
	}

	score := 0.0
	resultTitle := result.Title
	resultDuration := int(result.Duration)

	if !criteria.OriginalHasLiveTag && TitleContainsLiveKeyword(resultTitle) {
		log.Printf("INFO: search_filter_rejected title=%q reason=live_keyword", resultTitle)
		return math.MaxFloat64
	}

	// Duration penalty: large differences indicate a wrong version
	if criteria.ExpectedDurationS > 0 && resultDuration > 0 {
		diff := math.Abs(float64(resultDuration - criteria.ExpectedDurationS))
		threshold := math.Max(float64(criteria.ExpectedDurationS)*0.30, 15)
		if diff > threshold {
			log.Printf("INFO: search_filter_rejected title=%q reason=duration_mismatch expected=%ds got=%ds",
				resultTitle, criteria.ExpectedDurationS, resultDuration)
			return math.MaxFloat64
		}
		// Proportional penalty for smaller deviations
		score += (diff / float64(criteria.ExpectedDurationS)) * 50
	}

	return score
}

// pickBestResult selects the best search result using criteria-based scoring.
// If criteria is nil, returns the first result.
func pickBestResult(results []ytDlpSearchResult, criteria *SearchCriteria) (ytDlpSearchResult, bool) {
	if len(results) == 0 {
		return ytDlpSearchResult{}, false
	}
	if criteria == nil {
		return results[0], true
	}

	bestScore := math.MaxFloat64
	bestIdx := -1
	for i, r := range results {
		s := scoreResult(r, criteria)
		if s < bestScore {
			bestScore = s
			bestIdx = i
		}
	}
	if bestIdx < 0 || bestScore == math.MaxFloat64 {
		return ytDlpSearchResult{}, false
	}
	return results[bestIdx], true
}