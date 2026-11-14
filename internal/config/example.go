package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const exampleFileName = ".switchlet.example.yaml"

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateExampleEnvironmentName enforces portable, unambiguous export names.
// Existing configuration loading remains backward compatible.
func ValidateExampleEnvironmentName(name string) error {
	if !environmentNamePattern.MatchString(name) {
		return fmt.Errorf("environment name must match [A-Za-z_][A-Za-z0-9_]*")
	}
	return nil
}

// PreparedExample contains sanitized output and the destination seen in review.
// It cannot be used to write literals or copy source YAML comments.
type PreparedExample struct {
	path           string
	contents       []byte
	previous       []byte
	info           fs.FileInfo
	protectedPaths []string
}

func (example PreparedExample) Path() string           { return example.path }
func (example PreparedExample) ReplacesExisting() bool { return example.info != nil }

// PrepareExample serializes fresh validated schema data and captures any existing
// example for explicit replacement review. It never resolves environment values.
func PrepareExample(projectRoot, activeConfigPath string, targets []Target, profiles []Profile) (PreparedExample, error) {
	if strings.TrimSpace(activeConfigPath) == "" {
		return PreparedExample{}, fmt.Errorf("active configuration path must be set")
	}
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return PreparedExample{}, fmt.Errorf("resolve example directory: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return PreparedExample{}, err
	}
	if !filepath.IsAbs(activeConfigPath) {
		activeConfigPath = filepath.Join(root, activeConfigPath)
	}
	normalized, err := ValidateDraft(projectRoot, targets, profiles)
	if err != nil {
		return PreparedExample{}, err
	}
	names := make(map[string]string)
	for _, profile := range normalized.Profiles {
		for _, value := range profile.Values {
			if value.Value != nil || value.ValueFromEnv == nil {
				return PreparedExample{}, fmt.Errorf("example profile %q target %q must use an environment reference", profile.Name, value.Target)
			}
			name := *value.ValueFromEnv
			if err := ValidateExampleEnvironmentName(name); err != nil {
				return PreparedExample{}, fmt.Errorf("profile %q target %q: %w", profile.Name, value.Target, err)
			}
			folded := strings.ToUpper(name)
			if previous, ok := names[folded]; ok && previous != name {
				return PreparedExample{}, fmt.Errorf("environment names %q and %q differ only by case", previous, name)
			}
			names[folded] = name
		}
	}
	contents, err := marshalNamedTargetConfig(projectRoot, normalized.Targets, normalized.Profiles, nil)
	if err != nil {
		return PreparedExample{}, err
	}
	example := PreparedExample{path: filepath.Join(root, exampleFileName), contents: contents, protectedPaths: []string{activeConfigPath}}
	for _, target := range normalized.Targets {
		example.protectedPaths = append(example.protectedPaths, target.File)
	}
	if _, err := loadConfigContents(filepath.Join(root, configFileName), contents); err != nil {
		return PreparedExample{}, fmt.Errorf("verify example configuration: %w", err)
	}
	if err := example.validateDestination(); err != nil {
		return PreparedExample{}, err
	}
	example.previous, example.info, err = readExampleDestination(example.path)
	if err != nil {
		return PreparedExample{}, err
	}
	return example, nil
}

// Commit writes the reviewed snapshot only if the example still has the reviewed
// identity and bytes. A new example uses exclusive creation to prevent overwrite.
func (example PreparedExample) Commit() error {
	if example.path == "" || len(example.contents) == 0 {
		return fmt.Errorf("example must be prepared before export")
	}
	if err := example.validateDestination(); err != nil {
		return err
	}
	current, info, err := readExampleDestination(example.path)
	if err != nil {
		return err
	}
	if (info == nil) != (example.info == nil) || (info != nil && (!os.SameFile(info, example.info) || info.Mode() != example.info.Mode() || !bytes.Equal(current, example.previous))) {
		return fmt.Errorf("example changed since review; review export again")
	}
	if info != nil {
		return writePreparedReplacement(example.path, example.contents, info.Mode().Perm())
	}
	return createExampleFile(example.path, example.contents)
}

func (example PreparedExample) validateDestination() error {
	parent, err := filepath.EvalSymlinks(filepath.Dir(example.path))
	if err != nil {
		return err
	}
	if parent != filepath.Dir(example.path) {
		return fmt.Errorf("example directory changed since review")
	}
	destinationInfo, err := os.Lstat(example.path)
	if err == nil && !destinationInfo.Mode().IsRegular() {
		return fmt.Errorf("example destination must be a regular file, not a symlink or special file")
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, path := range example.protectedPaths {
		canonical, err := resolveExampleAliasPath(path)
		if err != nil {
			return fmt.Errorf("check export alias: %w", err)
		}
		info, statErr := os.Stat(path)
		if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
			return fmt.Errorf("check export alias: %w", statErr)
		}
		if filepath.Clean(canonical) == example.path || (info != nil && destinationInfo != nil && os.SameFile(info, destinationInfo)) {
			return fmt.Errorf("example destination aliases the active configuration or a managed target")
		}
	}
	return nil
}

// Resolve a missing final component as well as dangling leaf symlinks. A managed
// path can already point at the not-yet-created example through such a link.
func resolveExampleAliasPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for range 40 {
		parent, err := filepath.EvalSymlinks(filepath.Dir(path))
		if errors.Is(err, fs.ErrNotExist) {
			return filepath.Clean(path), nil
		}
		if err != nil {
			return "", err
		}
		path = filepath.Join(parent, filepath.Base(path))
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return path, nil
		}
		link, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(parent, link)
		}
		path = filepath.Clean(link)
	}
	return "", fmt.Errorf("too many symbolic links in protected path")
}

func readExampleDestination(path string) ([]byte, fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("example destination must be a regular file, not a symlink or special file")
	}
	contents, err := os.ReadFile(path)
	return contents, info, err
}

func createExampleFile(path string, contents []byte) (returnErr error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create example: %w", err)
	}
	defer func() {
		closeErr := file.Close()
		if returnErr == nil {
			returnErr = closeErr
		}
		if returnErr != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(contents); err != nil {
		return err
	}
	return file.Sync()
}
