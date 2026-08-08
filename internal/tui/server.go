package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ServerStats struct {
	Repos   int64
	Files   int64
	Chunks  int64
	Symbols int64
	Refs    int64
}

type StatsFunc func(ctx context.Context) (ServerStats, error)

type ServerModel struct {
	addr    string
	started time.Time
	fetch   StatsFunc
	stats   ServerStats
	has     bool
	err     string
}

func NewServerModel(addr string, fetch StatsFunc) ServerModel {
	return ServerModel{
		addr:    addr,
		started: time.Now(),
		fetch:   fetch,
	}
}

func (m ServerModel) Init() tea.Cmd {
	return m.refreshCmd(0)
}

func (m ServerModel) refreshCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return statsMsg(fetchStats(m.fetch))
	})
}

func fetchStats(fetch StatsFunc) statsMsg {
	if fetch == nil {
		return statsMsg{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	s, err := fetch(ctx)
	return statsMsg{s, err}
}

type statsMsg struct {
	stats ServerStats
	err   error
}

func (m ServerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil

	case statsMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.stats = msg.stats
			m.has = true
			m.err = ""
		}
		return m, m.refreshCmd(time.Second * 2)
	}

	return m, nil
}

func (m ServerModel) View() string {
	uptime := time.Since(m.started).Round(time.Second)

	rows := []struct {
		label string
		value string
	}{
		{"Repositories", humanInt(m.stats.Repos)},
		{"Files", humanInt(m.stats.Files)},
		{"Chunks", humanInt(m.stats.Chunks)},
		{"Symbols", humanInt(m.stats.Symbols)},
		{"References", humanInt(m.stats.Refs)},
	}

	placeholder := ""
	if !m.has {
		placeholder = "…"
	}

	maxLabel := 0
	for _, r := range rows {
		if len(r.label) > maxLabel {
			maxLabel = len(r.label)
		}
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf("  %s  %s\n\n", SuccessStyle.Render("●"), TitleStyle.Render("Relith server")))
	url := "http://" + m.addr
	if m.err != "" {
		body.WriteString(ErrorStyle.Render("  "+url) + "  " + MutedStyle.Render(m.err) + "\n\n")
	} else {
		body.WriteString(InfoStyle.Render("  " + hyperlink(url, m.addr)) + "\n\n")
	}

	for _, r := range rows {
		val := r.value
		if placeholder != "" {
			val = placeholder
		}
		fmt.Fprintf(&body, "  %-*s  %s\n", maxLabel, r.label, InfoStyle.Render(val))
	}

	body.WriteString("\n  " + MutedStyle.Render(fmt.Sprintf("uptime %s · press Ctrl+C to stop", uptime.String())) + "\n")

	return body.String()
}

func hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func humanInt(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

var _ tea.Model = ServerModel{}