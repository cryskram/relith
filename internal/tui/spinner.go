package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbles/spinner"
)

type doneMsg struct{}

type spinnerModel struct {
	spinner  spinner.Model
	label    string
	done     bool
	err      error
	result   string
	doneCh   chan error
}

func NewSpinner(label string, doneCh chan error) tea.Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot
	return spinnerModel{spinner: s, label: label, doneCh: doneCh}
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		err := <-m.doneCh
		if err != nil {
			return errMsg{err}
		}
		return doneMsg{}
	})
}

type errMsg struct{ err error }

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case doneMsg:
		m.done = true
		return m, tea.Quit

	case errMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m spinnerModel) View() string {
	if m.done {
		return ""
	}
	return "  " + m.spinner.View() + " " + InfoStyle.Render(m.label) + "\n"
}

var _ tea.Model = spinnerModel{}
