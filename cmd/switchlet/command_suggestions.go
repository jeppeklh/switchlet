package main

import (
	"strings"

	"github.com/jeppeklh/switchlet/internal/app"
)

func suggestedProfileName(profileName string, profiles []app.ProfileItem) string {
	candidates := make([]string, 0, len(profiles))
	for _, profileItem := range profiles {
		candidates = append(candidates, profileItem.Name)
	}

	return suggestedName(profileName, candidates)
}

func suggestedName(requestedName string, candidates []string) string {
	requested := strings.ToLower(strings.TrimSpace(requestedName))
	if requested == "" {
		return ""
	}

	bestDistance := -1
	bestSuggestion := ""
	bestMatches := 0
	for _, candidateName := range candidates {
		candidate := strings.ToLower(candidateName)
		distance := levenshteinDistance(requested, candidate)
		if distance > suggestionThreshold(requested, candidate) {
			continue
		}
		if bestDistance < 0 || distance < bestDistance {
			bestDistance = distance
			bestSuggestion = candidateName
			bestMatches = 1
			continue
		}
		if distance == bestDistance {
			bestMatches++
		}
	}

	if bestMatches == 1 {
		return bestSuggestion
	}

	return ""
}

func suggestionThreshold(requested string, candidate string) int {
	maxLength := len([]rune(requested))
	if candidateLength := len([]rune(candidate)); candidateLength > maxLength {
		maxLength = candidateLength
	}
	if maxLength <= 3 {
		return 0
	}
	if maxLength <= 6 {
		return 1
	}

	return 2
}

func levenshteinDistance(left string, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	previous := make([]int, len(rightRunes)+1)
	current := make([]int, len(rightRunes)+1)

	for index := range previous {
		previous[index] = index
	}

	for leftIndex, leftRune := range leftRunes {
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range rightRunes {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}

			current[rightIndex+1] = minInt(
				current[rightIndex]+1,
				previous[rightIndex+1]+1,
				previous[rightIndex]+cost,
			)
		}

		previous, current = current, previous
	}

	return previous[len(rightRunes)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}

	return minimum
}
