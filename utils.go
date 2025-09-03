package main

import (
	"regexp"
	"strings"
)

func cleanTrackTitle(title string) string {
	// Remove featuring artists
	re := regexp.MustCompile(`\s*\(feat\..*\)`) // Matches (feat. ...) and variations
	title = re.ReplaceAllString(title, "")

	// Remove common suffixes like (Live), (Remix), etc.
	re = regexp.MustCompile(`\s*\(.*(Live|Remix|Remaster|Acoustic|Version|Edit|Radio|Mix).*\)`) // Matches common suffixes
	title = re.ReplaceAllString(title, "")

	// Standardize delimiters
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.ReplaceAll(title, ":", " ")

	// Remove extra whitespace
	title = strings.Join(strings.Fields(title), " ")

	return strings.TrimSpace(title)
}

// removeDuplicateStrings removes duplicate strings from a slice while preserving order.
func removeDuplicateStrings(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if !encountered[elements[v]] {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}