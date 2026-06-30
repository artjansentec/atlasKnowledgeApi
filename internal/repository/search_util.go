package repository

import "strings"

const (
	SearchLimitSections = 50
	SearchLimitLessons  = 50
	SearchLimitUpdates  = 50
	SearchUpdatesMaxAgeDays = 90
)

func likePattern(query string) string {
	return "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
}
