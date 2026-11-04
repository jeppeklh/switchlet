package config

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// ValueType is the declared type of a managed scalar, independent of file format.
type ValueType string

const (
	ValueTypeString  ValueType = "string"
	ValueTypeBoolean ValueType = "boolean"
	ValueTypeInteger ValueType = "integer"
	ValueTypeNumber  ValueType = "number"
)

// Scalar is a validated value with an explicit type and exact canonical text.
// Text is exposed intentionally; callers must keep it out of ordinary output.
type Scalar struct {
	kind ValueType
	text string
}

func (value Scalar) Type() ValueType { return value.kind }
func (value Scalar) Text() string    { return value.text }

// EffectiveValueType preserves the string semantics of older configurations.
func EffectiveValueType(kind ValueType) ValueType {
	if kind == "" {
		return ValueTypeString
	}
	return kind
}

var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
var integerPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

// ParseScalar validates text without exposing it in errors. Numbers use exact
// decimal arithmetic; canonical text is also the equality representation.
func ParseScalar(kind ValueType, text string) (Scalar, error) {
	kind = EffectiveValueType(kind)
	value := Scalar{kind: kind, text: text}
	switch kind {
	case ValueTypeString:
	case ValueTypeBoolean:
		if text != "true" && text != "false" {
			return Scalar{}, fmt.Errorf("expected boolean true or false")
		}
	case ValueTypeInteger:
		if !integerPattern.MatchString(text) {
			return Scalar{}, fmt.Errorf("expected decimal integer")
		}
		integer, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return Scalar{}, fmt.Errorf("integer must fit signed 64-bit range")
		}
		value.text = strconv.FormatInt(integer, 10)
	case ValueTypeNumber:
		if len(text) > 512 || !decimalPattern.MatchString(text) {
			return Scalar{}, fmt.Errorf("expected finite decimal number")
		}
		// Even zero can carry a huge exponent without ParseFloat reporting an
		// overflow. Bound it before exact rational arithmetic allocates powers.
		if index := strings.IndexAny(text, "eE"); index >= 0 {
			exponent, err := strconv.ParseInt(text[index+1:], 10, 32)
			if err != nil || exponent < -1024 || exponent > 1024 {
				return Scalar{}, fmt.Errorf("number exponent is outside the supported range")
			}
		}
		floating, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(floating, 0) || math.IsNaN(floating) {
			return Scalar{}, fmt.Errorf("number is outside the finite supported range")
		}
		rational, ok := new(big.Rat).SetString(text)
		if !ok || (floating == 0 && rational.Sign() != 0) {
			return Scalar{}, fmt.Errorf("number is outside the finite supported range")
		}
		// At most 512 input characters and float64's finite exponent range bound
		// the exact decimal scale. No binary floating-point result is used as data.
		scale := len(text) + 324
		value.text = strings.TrimRight(strings.TrimRight(rational.FloatString(scale), "0"), ".")
		if value.text == "" || value.text == "-0" {
			value.text = "0"
		}
		if !strings.Contains(value.text, ".") {
			value.text += ".0"
		}
		if len(value.text) > 512 {
			return Scalar{}, fmt.Errorf("number exceeds the supported canonical decimal length")
		}
	default:
		return Scalar{}, fmt.Errorf("unsupported value type %q", kind)
	}
	return value, nil
}

// SchemaVersionForTargets keeps projects without explicit value types on Version 3.
func SchemaVersionForTargets(targets []Target) int {
	for _, target := range targets {
		if target.ValueType != "" {
			return typedTargetVersion
		}
	}
	return namedTargetVersion
}

// ValidateDraft validates the schema appropriate for the supplied targets.
func ValidateDraft(projectRoot string, targets []Target, profiles []Profile) (Config, error) {
	root, err := resolveProjectRoot(projectRoot)
	if err != nil {
		return Config{}, err
	}
	return validateNamedTargetConfig(SchemaVersionForTargets(targets), root, fileConfig{Targets: fileTargetsFromTargets(root, targets), Profiles: fileProfilesFromProfiles(profiles)})
}

func validateValueType(kind ValueType) error {
	switch EffectiveValueType(kind) {
	case ValueTypeString, ValueTypeBoolean, ValueTypeInteger, ValueTypeNumber:
		return nil
	}
	return fmt.Errorf("unsupported value type %q", kind)
}
