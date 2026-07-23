package profile

import "strings"

const maskedConnectionStringValue = "****"

// MaskConnectionString returns a display-safe representation of a connection string.
func MaskConnectionString(connectionString string) string {
	segments := strings.Split(connectionString, ";")
	for index, segment := range segments {
		segments[index] = maskSegment(segment)
	}

	return strings.Join(segments, ";")
}

func maskSegment(segment string) string {
	key, _, found := strings.Cut(segment, "=")
	if !found {
		return segment
	}

	trimmedKey := strings.TrimSpace(key)
	if !strings.EqualFold(trimmedKey, "Password") && !strings.EqualFold(trimmedKey, "Pwd") {
		return segment
	}

	return key + "=" + maskedConnectionStringValue
}
