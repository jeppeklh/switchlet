package profile

import "strings"

const maskedManagedValue = "****"

// ManagedValueMaskContext describes user-controlled names around one managed value.
type ManagedValueMaskContext struct {
	TargetName              string
	Selector                string
	EnvironmentVariableName string
}

// MaskManagedValue returns a display-safe representation of one managed value.
func MaskManagedValue(value string, context ManagedValueMaskContext) string {
	if value == "" {
		return ""
	}
	_ = context

	return maskedManagedValue
}

// MaskConnectionString returns a display-safe representation of a connection string.
func MaskConnectionString(connectionString string) string {
	return maskConnectionString(connectionString)
}

func maskConnectionString(connectionString string) string {
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
	if !textIndicatesSensitive(trimmedKey) {
		return segment
	}

	return key + "=" + maskedManagedValue
}

func textIndicatesSensitive(text string) bool {
	lowerText := strings.ToLower(text)
	for _, term := range []string{"password", "passwd", "pwd", "secret", "token", "credential"} {
		if strings.Contains(lowerText, term) {
			return true
		}
	}

	for _, token := range identifierTokens(text) {
		if token == "key" {
			return true
		}
	}

	return false
}

func identifierTokens(text string) []string {
	tokens := make([]string, 0)
	var builder strings.Builder
	var previous byte
	for index := 0; index < len(text); index++ {
		current := text[index]
		if !isASCIIAlphaNumeric(current) {
			if builder.Len() > 0 {
				tokens = append(tokens, builder.String())
				builder.Reset()
			}
			previous = 0
			continue
		}

		next := byte(0)
		if index+1 < len(text) {
			next = text[index+1]
		}
		if builder.Len() > 0 && shouldSplitIdentifierToken(previous, current, next) {
			tokens = append(tokens, builder.String())
			builder.Reset()
		}
		builder.WriteByte(toLowerASCII(current))
		previous = current
	}
	if builder.Len() > 0 {
		tokens = append(tokens, builder.String())
	}

	return tokens
}

func shouldSplitIdentifierToken(previous byte, current byte, next byte) bool {
	if (isASCIILower(previous) || isASCIIDigit(previous)) && isASCIIUpper(current) {
		return true
	}

	return isASCIIUpper(previous) && isASCIIUpper(current) && isASCIILower(next)
}

func isASCIIAlphaNumeric(value byte) bool {
	return isASCIILower(value) || isASCIIUpper(value) || isASCIIDigit(value)
}

func isASCIILower(value byte) bool {
	return value >= 'a' && value <= 'z'
}

func isASCIIUpper(value byte) bool {
	return value >= 'A' && value <= 'Z'
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func toLowerASCII(value byte) byte {
	if isASCIIUpper(value) {
		return value + ('a' - 'A')
	}

	return value
}
