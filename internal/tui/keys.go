package tui

import tea "github.com/charmbracelet/bubbletea"

const (
	keyUp      = "up"
	keyDown    = "down"
	keyPageUp  = "pgup"
	keyPageDn  = "pgdown"
	keyHome    = "home"
	keyEnd     = "end"
	keyMoveUp  = "k"
	keyMoveDn  = "j"
	keyEnter   = "enter"
	keyInspect = "i"
	keyStatus  = "s"
	keyDiff    = "d"
	keyReveal  = "v"
	keyRefresh = "r"
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
