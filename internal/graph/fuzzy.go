package graph

import (
	"strings"
	"unicode"
)

// FuzzyMatch checks if two names are similar enough to be a potential match.
// Returns the match score (0-1) and reason.
func FuzzyMatch(name1, name2 string) (float64, string) {
	n1 := strings.ToUpper(strings.TrimSpace(name1))
	n2 := strings.ToUpper(strings.TrimSpace(name2))

	if n1 == "" || n2 == "" {
		return 0, ""
	}

	// Exact match
	if n1 == n2 {
		return 1.0, "exact_match"
	}

	// Substring containment
	if len(n2) >= 4 && strings.Contains(n1, n2) {
		return 0.8, "substring_contains"
	}
	if len(n1) >= 4 && strings.Contains(n2, n1) {
		return 0.8, "substring_contains"
	}

	// Shared prefix >= 4 chars
	prefixLen := sharedPrefixLength(n1, n2)
	if prefixLen >= 4 {
		minLen := len(n1)
		if len(n2) < minLen {
			minLen = len(n2)
		}
		score := float64(prefixLen) / float64(minLen)
		if score >= 0.5 {
			return score, "shared_prefix"
		}
	}

	// COBOL abbreviation pattern: drop vowels, truncate to 8
	abbr1 := cobolAbbreviate(n1)
	abbr2 := cobolAbbreviate(n2)
	if len(abbr1) >= 4 && abbr1 == abbr2 {
		return 0.7, "cobol_abbreviation"
	}

	// Levenshtein normalized
	dist := levenshteinDistance(n1, n2)
	maxLen := len(n1)
	if len(n2) > maxLen {
		maxLen = len(n2)
	}
	if maxLen == 0 {
		return 0, ""
	}
	normalized := 1.0 - float64(dist)/float64(maxLen)
	if normalized >= 0.6 {
		return normalized, "levenshtein"
	}

	return 0, ""
}

// cobolAbbreviate applies COBOL naming conventions: drop vowels, truncate to 8 chars.
func cobolAbbreviate(name string) string {
	// Strip common suffixes
	for _, suffix := range []string{"SERVICE", "CONTROLLER", "ADAPTER", "GATEWAY", "HANDLER", "MANAGER"} {
		name = strings.TrimSuffix(name, suffix)
	}

	// Drop vowels (except first character)
	var result []rune
	for i, r := range name {
		if i == 0 || !isVowel(r) {
			result = append(result, r)
		}
	}

	// Truncate to 8 chars (COBOL convention)
	if len(result) > 8 {
		result = result[:8]
	}
	return string(result)
}

func isVowel(r rune) bool {
	r = unicode.ToUpper(r)
	return r == 'A' || r == 'E' || r == 'I' || r == 'O' || r == 'U'
}

func sharedPrefixLength(a, b string) int {
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return maxLen
}

func levenshteinDistance(a, b string) int {
	la := len(a)
	lb := len(b)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use single-row optimization
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}
