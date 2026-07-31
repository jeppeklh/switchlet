package configeditor

import tea "github.com/charmbracelet/bubbletea"

func isMoveUpKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyUp || isRuneKey(message, 'k')
}

func isMoveDownKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyDown || isRuneKey(message, 'j')
}

func isFirstKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyHome || isRuneKey(message, 'g')
}

func isLastKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyEnd || isRuneKey(message, 'G')
}

func isOpenKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyEnter || isRuneKey(message, 'l')
}

func isBackKey(message tea.KeyMsg) bool {
	return message.Type == tea.KeyEsc || isRuneKey(message, 'h')
}

func isQuitKey(message tea.KeyMsg) bool {
	return isRuneKey(message, 'q')
}

func isRuneKey(message tea.KeyMsg, key rune) bool {
	return message.Type == tea.KeyRunes && message.String() == string(key)
}
