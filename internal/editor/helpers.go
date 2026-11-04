package editor

import (
	"encoding/json"
	"fmt"
	"github.com/jeppeklh/switchlet/internal/config"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

func statRegularTargetFile(targetPath string) (fs.FileInfo, error) {
	if targetPath == "" {
		return nil, fmt.Errorf("target path must be set")
	}

	targetInfo, err := os.Lstat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("stat target file %q: %w", targetPath, err)
	}
	if err := validateRegularTargetFileInfo(targetPath, targetInfo); err != nil {
		return nil, err
	}

	return targetInfo, nil
}

func validateRegularTargetFileInfo(targetPath string, targetInfo fs.FileInfo) error {
	mode := targetInfo.Mode()

	switch {
	case mode&fs.ModeSymlink != 0:
		return fmt.Errorf("target file %q is a symlink; Switchlet only supports regular files", targetPath)
	case targetInfo.IsDir():
		return fmt.Errorf("target file %q is a directory", targetPath)
	case mode.IsRegular():
		return nil
	case mode&fs.ModeNamedPipe != 0:
		return fmt.Errorf("target file %q is a named pipe; Switchlet only supports regular files", targetPath)
	case mode&fs.ModeSocket != 0:
		return fmt.Errorf("target file %q is a socket; Switchlet only supports regular files", targetPath)
	case mode&fs.ModeDevice != 0 && mode&fs.ModeCharDevice != 0:
		return fmt.Errorf("target file %q is a character device; Switchlet only supports regular files", targetPath)
	case mode&fs.ModeDevice != 0:
		return fmt.Errorf("target file %q is a device; Switchlet only supports regular files", targetPath)
	default:
		return fmt.Errorf("target file %q is not a regular file; Switchlet only supports regular files", targetPath)
	}
}

func nativeScalar(value any, kind config.ValueType) (config.Scalar, error) {
	kind = config.EffectiveValueType(kind)
	var text string
	switch value := value.(type) {
	case string:
		if kind != config.ValueTypeString {
			return config.Scalar{}, fmt.Errorf("expected %s scalar", kind)
		}
		text = value
	case bool:
		if kind != config.ValueTypeBoolean {
			return config.Scalar{}, fmt.Errorf("expected %s scalar", kind)
		}
		text = strconv.FormatBool(value)
	case json.Number:
		if kind != config.ValueTypeInteger && kind != config.ValueTypeNumber {
			return config.Scalar{}, fmt.Errorf("expected %s scalar", kind)
		}
		text = string(value)
	default:
		return config.Scalar{}, fmt.Errorf("expected %s scalar", kind)
	}
	return config.ParseScalar(kind, text)
}
func encodedScalar(value string, kind config.ValueType) ([]byte, error) {
	scalar, err := config.ParseScalar(kind, value)
	if err != nil {
		return nil, err
	}
	if scalar.Type() == config.ValueTypeString {
		return json.Marshal(scalar.Text())
	}
	return []byte(scalar.Text()), nil
}

func nativeValueType(value any) config.ValueType {
	switch value := value.(type) {
	case string:
		return ""
	case bool:
		return config.ValueTypeBoolean
	case json.Number:
		if _, err := config.ParseScalar(config.ValueTypeInteger, string(value)); err == nil {
			return config.ValueTypeInteger
		}
		return config.ValueTypeNumber
	}
	return "unsupported"
}

// DetectTargetValueType infers the existing native scalar for setup workflows.
// Dotenv has no native types and starts as string unless explicitly configured.
func DetectTargetValueType(target config.Target) (config.ValueType, error) {
	if target.Type == config.TargetTypeDotenv {
		return "", ValidateDotenvTarget(target.File, target.Key)
	}
	var original error
	for _, kind := range []config.ValueType{"", config.ValueTypeBoolean, config.ValueTypeInteger, config.ValueTypeNumber} {
		target.ValueType = kind
		// ReadTargetValues also enforces normal path/alias/regular-file safeguards.
		if _, err := ReadTargetValues([]config.Target{target}); err == nil {
			return kind, nil
		} else if original == nil {
			original = err
		}
	}
	return "", original
}

// sourceScalar accepts the native numeric spelling of a parsed source scalar.
// Profile/environment entry uses ParseScalar directly and remains strict decimal.
func sourceScalar(expected, source config.ValueType, text string) (config.Scalar, error) {
	switch source {
	case config.ValueTypeBoolean:
		text = strings.ToLower(text)
	case config.ValueTypeInteger, config.ValueTypeNumber:
		text = strings.TrimPrefix(strings.ReplaceAll(text, "_", ""), "+")
		if source == config.ValueTypeInteger && (strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0o") || strings.HasPrefix(text, "0b") || strings.HasPrefix(text, "-0x") || strings.HasPrefix(text, "-0o") || strings.HasPrefix(text, "-0b")) {
			integer, err := strconv.ParseInt(text, 0, 64)
			if err != nil {
				return config.Scalar{}, fmt.Errorf("integer must fit signed 64-bit range")
			}
			text = strconv.FormatInt(integer, 10)
		}
		if strings.HasPrefix(text, ".") {
			text = "0" + text
		}
		if strings.HasPrefix(text, "-.") {
			text = "-0" + text[1:]
		}
		if strings.HasSuffix(text, ".") {
			text += "0"
		}
	}
	return config.ParseScalar(expected, text)
}
