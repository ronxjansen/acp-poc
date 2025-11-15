package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	caretStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCaret)).
			Bold(true)

	placeholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorPlaceholder))
)

// InputBox handles all input box logic and rendering
type InputBox struct {
	value       string
	cursor      int
	placeholder string
}

// NewInputBox creates a new input box
func NewInputBox(placeholder string) InputBox {
	return InputBox{
		value:       "",
		cursor:      0,
		placeholder: placeholder,
	}
}

// Update handles keyboard input for the input box
func (i *InputBox) Update(msg tea.KeyMsg) (bool, string) {
	switch msg.String() {
	case "enter":
		if i.value != "" {
			submitted := i.value
			i.Clear()
			return true, submitted
		}
		return false, ""

	case "backspace":
		if i.cursor > 0 {
			i.value = i.value[:i.cursor-1] + i.value[i.cursor:]
			i.cursor--
		}
		return false, ""

	case "left":
		if i.cursor > 0 {
			i.cursor--
		}
		return false, ""

	case "right":
		if i.cursor < len(i.value) {
			i.cursor++
		}
		return false, ""

	case "home":
		i.cursor = 0
		return false, ""

	case "end":
		i.cursor = len(i.value)
		return false, ""

	default:
		// Handle regular character input
		if len(msg.String()) == 1 {
			i.value = i.value[:i.cursor] + msg.String() + i.value[i.cursor:]
			i.cursor++
		}
		return false, ""
	}
}

// View renders the input box
func (i InputBox) View() string {
	caret := caretStyle.Render(">")

	var inputText string
	if i.value == "" {
		inputText = placeholderStyle.Render(i.placeholder)
	} else {
		// Show cursor as █ block
		if i.cursor < len(i.value) {
			inputText = i.value[:i.cursor] + "█" + i.value[i.cursor:]
		} else {
			inputText = i.value + "█"
		}
	}

	return caret + " " + inputText
}

// Clear resets the input box
func (i *InputBox) Clear() {
	i.value = ""
	i.cursor = 0
}

// Value returns the current input value
func (i InputBox) Value() string {
	return i.value
}

// IsEmpty returns whether the input is empty
func (i InputBox) IsEmpty() bool {
	return i.value == ""
}
