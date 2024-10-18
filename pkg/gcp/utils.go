package gcp

import (
	"fmt"
	"regexp"
)

func extractResourceRegion(urlStr string) (string, error) {
	// Define the regex pattern
	re := regexp.MustCompile(`regions/([^/]+)`)

	// Find the matches
	matches := re.FindStringSubmatch(urlStr)
	if len(matches) < 2 {
		return "", fmt.Errorf("no matches found in URL")
	}

	// Extract the region from the matches
	region := matches[1]

	return region, nil
}
