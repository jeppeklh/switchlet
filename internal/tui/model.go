package tui

import (
	"strings"

	"github.com/charmbracelet/bubbletea"

	"github.com/jeppeklh/switchlet/internal/app"
)

type viewState int

const (
	listState viewState = iota
	searchState
	inspectState
	confirmState
	errorState
	successState
	statusLoadingState
	statusReadyState
	diffLoadingState
	diffReadyState
	comparisonErrorState
)

type comparisonRequestKind int

const (
	comparisonRequestNone comparisonRequestKind = iota
	comparisonRequestStatus
	comparisonRequestDiff
)

// Model is the Bubble Tea model for the Switchlet terminal interface.
type Model struct {
	application       app.Application
	projectRoot       string
	profiles          []app.ProfileItem
	cursor            int
	width             int
	height            int
	state             viewState
	profileFilter     string
	searchInput       string
	searchCursor      int
	searchStartCursor int
	scrollOffset      int
	valuesVisible     bool
	recoverableError  RecoverableError
	successResult     *app.Result
	applyingProfile   string
	applyExits        bool
	applyRequestID    int
	confirmExits      bool
	currentProfiles   map[string]struct{}
	currentRequestID  int
	configRequested   bool

	statusComparison      *app.StatusComparison
	diffPreview           *app.ManagedPatchPreview
	comparisonError       RecoverableError
	comparisonRequestID   int
	comparisonRequestKind comparisonRequestKind
	comparisonProfileName string
}

// New creates the terminal model.
func New(application app.Application) Model {
	return NewWithProjectRoot(application, "")
}

// NewWithProjectRoot creates the terminal model with project-relative display context.
func NewWithProjectRoot(application app.Application, projectRoot string) Model {
	model := Model{application: application, projectRoot: projectRoot, currentRequestID: 1}
	model.refreshProfiles()

	return model
}

// NewWithSelectedProfile creates the terminal model with the requested profile selected when it exists.
func NewWithSelectedProfile(application app.Application, profileName string) Model {
	return NewWithSelectedProfileAndProjectRoot(application, profileName, "")
}

// NewWithSelectedProfileAndProjectRoot creates the terminal model with selection and display context.
func NewWithSelectedProfileAndProjectRoot(application app.Application, profileName string, projectRoot string) Model {
	model := NewWithProjectRoot(application, projectRoot)
	model.selectProfileByName(profileName)

	return model
}

// NewReloadError creates a focused error model for a failed post-save reload.
func NewReloadError(err error) Model {
	return Model{
		state: errorState,
		recoverableError: RecoverableError{
			Problem:  "Configuration was saved, but Switchlet could not reload it.",
			Reason:   err.Error(),
			Recovery: "Fix .switchlet.yaml, then run Switchlet again.",
			Cause:    err,
		},
	}
}

// NewConfigOpenError creates a focused error model for a failed config-editor open.
func NewConfigOpenError(err error) Model {
	return Model{
		state: errorState,
		recoverableError: RecoverableError{
			Problem:  "Switchlet could not open the configuration editor.",
			Reason:   err.Error(),
			Recovery: "Fix .switchlet.yaml, then try again.",
			Cause:    err,
		},
	}
}

// ConfigRequested reports whether the main picker exited to open the config editor.
func (model Model) ConfigRequested() bool {
	return model.configRequested
}

// SelectedProfileName returns the currently selected profile name.
func (model Model) SelectedProfileName() (string, bool) {
	selectedProfile, ok := model.selectedProfile()
	if !ok {
		return "", false
	}

	return selectedProfile.Name, true
}

// Init starts the Bubble Tea model.
func (model Model) Init() tea.Cmd {
	if model.state == errorState {
		return nil
	}

	return detectCurrentProfile(model.application, model.currentRequestID)
}

func (model *Model) refreshProfiles() {
	model.profiles = model.application.Profiles()
	model.clampCursorToVisibleProfiles()
}

func (model *Model) clampCursorToVisibleProfiles() {
	if len(model.profiles) == 0 {
		model.cursor = 0
		return
	}

	if model.cursor < 0 {
		model.cursor = 0
	}
	if model.cursor >= len(model.profiles) {
		model.cursor = len(model.profiles) - 1
	}

	visibleIndices := model.filteredProfileIndices()
	if len(visibleIndices) == 0 {
		return
	}
	for _, profileIndex := range visibleIndices {
		if profileIndex == model.cursor {
			return
		}
	}

	model.cursor = visibleIndices[0]
}

func (model *Model) selectProfileByName(profileName string) {
	if profileName == "" {
		return
	}

	for index, profile := range model.profiles {
		if profile.Name == profileName {
			model.cursor = index
			return
		}
	}
}

func (model Model) selectedProfile() (app.ProfileItem, bool) {
	if len(model.profiles) == 0 || model.cursor < 0 || model.cursor >= len(model.profiles) {
		return app.ProfileItem{}, false
	}
	selectedProfile := model.profiles[model.cursor]
	if !model.profileMatchesActiveFilter(selectedProfile) {
		return app.ProfileItem{}, false
	}

	return selectedProfile, true
}

func (model Model) activeProfileFilter() string {
	if model.state == searchState {
		return strings.TrimSpace(model.searchInput)
	}

	return model.profileFilter
}

func (model Model) hasActiveProfileFilter() bool {
	return normalizeProfileFilter(model.activeProfileFilter()) != ""
}

func (model Model) filteredProfileIndices() []int {
	filter := normalizeProfileFilter(model.activeProfileFilter())
	indices := make([]int, 0, len(model.profiles))
	for index, profile := range model.profiles {
		if filter == "" || profileNameMatchesFilter(profile.Name, filter) {
			indices = append(indices, index)
		}
	}

	return indices
}

func (model Model) profileMatchesActiveFilter(profile app.ProfileItem) bool {
	filter := normalizeProfileFilter(model.activeProfileFilter())
	return filter == "" || profileNameMatchesFilter(profile.Name, filter)
}

func normalizeProfileFilter(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func profileNameMatchesFilter(profileName string, normalizedFilter string) bool {
	return strings.Contains(strings.ToLower(profileName), normalizedFilter)
}
