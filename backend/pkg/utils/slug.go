/*
  File: slug.go
  Purpose: Utility functions for generating URL-safe slugs from strings.
  Author: CodeTextor project
  Notes: Used to generate immutable slugs for project database filenames.
*/

package utils

import (
	"regexp"
	"strings"
)

var (
	// nonAlphanumeric matches any character that's not alphanumeric or hyphen
	nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]+`)

	// multipleHyphens matches consecutive hyphens
	multipleHyphens = regexp.MustCompile(`-+`)
)

// GenerateSlug converts a string to a URL-safe slug.
// Rules:
// - Converts to lowercase
// - Replaces spaces and underscores with hyphens
// - Removes all non-alphanumeric characters except hyphens
// - Collapses multiple consecutive hyphens to single hyphen
// - Trims leading/trailing hyphens
// - Returns empty string if input is empty or results in empty slug
//
// Examples:
//   "My Awesome Project!" -> "my-awesome-project"
//   "Code_Textor 2.0" -> "code-textor-2-0"
//   "Test___Project" -> "test-project"
//   "!!!" -> ""
func GenerateSlug(s string) string {
	// Convert to lowercase
	slug := strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove all non-alphanumeric characters except hyphens
	slug = nonAlphanumeric.ReplaceAllString(slug, "")

	// Collapse multiple consecutive hyphens
	slug = multipleHyphens.ReplaceAllString(slug, "-")

	// Trim leading and trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}
