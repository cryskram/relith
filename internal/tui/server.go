package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type ServerModel struct {
	spinner spinner.Model
	addr    string
	started time.Time
	done    bool
}

func NewServerModel(addr string) ServerModel {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot
	return ServerModel{
		spinner: s,
		addr:    addr,
		started: time.Now(),
	}
}

func (m ServerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m ServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m ServerModel) View() string {
	if m.done {
		return ""
	}

	uptime := time.Since(m.started).Round(time.Second)

	url := fmt.Sprintf("http://%s", m.addr)

	content := fmt.Sprintf("\n  %s  %s\n\n",
		m.spinner.View(),
		TitleStyle.Render("Relith Server"))

	content += BoxStyle.Render(
		fmt.Sprintf("  %s\n\n  %s\n\n  %s  %s\n",
			HighlightStyle.Render("Dashboard & API"),
			InfoStyle.Render(url),
			MutedStyle.Render("uptime"),
			SuccessStyle.Render(uptime.String()),
		),
	)

	content += fmt.Sprintf("\n  %s\n", MutedStyle.Render("Press Ctrl+C to stop"))

	return content
}

var _ tea.Model = ServerModel{}
