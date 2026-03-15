package audio

import (
	"context"
	"regexp"
	"strings"
	"sync"
)

var featPattern = regexp.MustCompile(`\s*[\(\[](feat\.?|ft\.?|featuring)[^\)\]]*[\)\]]`)
var specialCharsPattern = regexp.MustCompile(`[&+!@#$%^*(){}|\\<>]`)

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

	// Original: "artist - title"
	add(artist + " - " + title)

	// Strip feat/ft from title
	strippedTitle := featPattern.ReplaceAllString(title, "")
	strippedTitle = strings.TrimSpace(strippedTitle)
	if strippedTitle != title {
		add(artist + " - " + strippedTitle)
	}

	// Remove special characters from title
	cleanTitle := specialCharsPattern.ReplaceAllString(title, " ")
	cleanTitle = collapseSpaces(cleanTitle)
	if cleanTitle != title {
		add(artist + " - " + cleanTitle)
	}

	// Title only (no artist)
	add(title)
	if strippedTitle != title {
		add(strippedTitle)
	}

	// Artist + simplified title (no dash separator)
	cleanArtist := specialCharsPattern.ReplaceAllString(artist, " ")
	cleanArtist = collapseSpaces(cleanArtist)
	add(cleanArtist + " " + cleanTitle)

	return variants
}

func collapseSpaces(s string) string {
	parts := strings.Fields(s)
	return strings.Join(parts, " ")
}

// searchResult carries the result from a parallel search goroutine.
type searchResult struct {
	url   string
	query string
	err   error
}

// SearchWithFallbacks tries multiple query variants in parallel across all providers.
// Returns the URL from the first variant that succeeds.
func (p *Provider) SearchWithFallbacks(ctx context.Context, variants []string) (string, error) {
	if len(variants) == 0 {
		return "", &SearchError{Message: "no search variants provided"}
	}

	// Fast path: single variant uses normal Search
	if len(variants) == 1 {
		return p.Search(ctx, variants[0])
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan searchResult, len(variants))
	var wg sync.WaitGroup

	for _, query := range variants {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			url, err := p.searchUncached(ctx, q)
			select {
			case results <- searchResult{url: url, query: q, err: err}:
			case <-ctx.Done():
			}
		}(query)
	}

	// Close results channel after all goroutines finish
	go func() {
		wg.Wait()
		close(results)
	}()

	var lastErr error
	for res := range results {
		if res.err == nil && res.url != "" {
			cancel()
			// Cache all variants with the winning URL
			for _, v := range variants {
				cacheKey := p.normalizeQuery(v)
				p.searchCache.Set(cacheKey, res.url)
			}
			return res.url, nil
		}
		if res.err != nil {
			lastErr = res.err
		}
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", &SearchError{Message: "No audio found across all search variants"}
}

// searchUncached searches without checking/populating cache (for parallel variant search).
func (p *Provider) searchUncached(ctx context.Context, query string) (string, error) {
	// Check cache first
	cacheKey := p.normalizeQuery(query)
	if cached := p.searchCache.Get(cacheKey); cached != nil {
		if url, ok := cached.(string); ok && url != "" {
			return url, nil
		}
	}

	for _, provider := range p.config.AudioProviders {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if limiter, ok := p.rateLimiters[provider]; ok {
			if err := limiter.WaitIfNeeded(ctx); err != nil {
				continue
			}
		}
		url, err := p.searchProvider(ctx, provider, query)
		if err == nil && url != "" {
			return url, nil
		}
	}
	return "", &SearchError{Message: "No audio found for: " + query}
}
