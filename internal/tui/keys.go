package tui

import tea "github.com/charmbracelet/bubbletea"

const (
	keyUp      = "up"
	keyDown    = "down"
	keyMoveUp  = "k"
	keyMoveDn  = "j"
	keyEnter   = "enter"
	keyInspect = "i"
	keyEscape  = "esc"
	keyConfirm = "y"
	keyCancel  = "n"
	keyQuit    = "q"
	keyCtrlC   = "ctrl+c"
)

func matchesKey(message tea.KeyMsg, keys ...string) bool {
	pressedKey := message.String()
	for _, key := range keys {
		if pressedKey == key {
			return true
		}
	}

	return false
}
