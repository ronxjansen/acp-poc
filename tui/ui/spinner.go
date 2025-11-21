package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// spinnerChars defines the character set for the spinner animation
// Includes 0-9, a-f (hex), and special characters for visual variety
var spinnerChars = []rune{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'a', 'b', 'c', 'd', 'e', 'f',
	'+', '.', '*', '&', '%', '#', '!',
}

// HexSpinner represents a hexadecimal loading indicator
type HexSpinner struct {
	positions [16]int  // Each position is an index into the character set
	frame     int
	chars     []rune   // Character set to randomly choose from
}

// TickMsg is sent on each spinner animation frame
type TickMsg time.Time

// NewHexSpinner creates a new hexadecimal spinner
func NewHexSpinner() HexSpinner {
	return HexSpinner{
		positions: [16]int{},
		frame:     0,
		chars:     spinnerChars,
	}
}

// Update updates the spinner state
func (s HexSpinner) Update(msg tea.Msg) (HexSpinner, tea.Cmd) {
	switch msg.(type) {
	case TickMsg:
		s.frame++
		// Randomly select a character for each position
		for i := range s.positions {
			s.positions[i] = rand.Intn(len(s.chars))
		}
		return s, tick()
	}
	return s, nil
}

// View renders the spinner
func (s HexSpinner) View() string {
	var hexChars strings.Builder

	// Build the character string using random characters from the set
	for _, idx := range s.positions {
		hexChars.WriteRune(s.chars[idx])
	}

	// Style with cyber/hacker aesthetic - green color
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")). // Bright green
		Bold(true)

	// Add brackets for tech feel
	return style.Render(fmt.Sprintf("[%s]", hexChars.String()))
}

// tick returns a command that sends a TickMsg after a short interval
func tick() tea.Cmd {
	return tea.Tick(70*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Init initializes the spinner and starts the tick
func (s HexSpinner) Init() tea.Cmd {
	return tick()
}
