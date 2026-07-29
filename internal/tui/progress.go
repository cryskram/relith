package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cryskram/relith/internal/indexer"
)

type eventMsg indexer.ProgressEvent

type model struct {
	spinner      spinner.Model
	prog         progress.Model
	sub          chan indexer.ProgressEvent
	phase        string
	filesFound   int
	filesIndexed int
	filesSkipped int
	filesError   int
	totalChunks  int
	elapsed      time.Duration
	err          error
	width        int
}

func NewProgress(sub chan indexer.ProgressEvent) tea.Model {
	s := spinner.New()
	s.Style = spinnerStyle
	s.Spinner = spinner.Dot

	p := progress.New(progress.WithSolidFill("#FF7700"), progress.WithWidth(50))

	return model{spinner: s, prog: p, sub: sub, phase: "walk"}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, waitForEvent(m.sub))
}

func waitForEvent(sub chan indexer.ProgressEvent) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-sub
		if !ok {
			return nil
		}
		return eventMsg(evt)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.prog.Width = clamp(msg.Width-12, 20, 120)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.err = fmt.Errorf("cancelled")
			return m, tea.Sequence(tea.Println(m.finalView()), tea.Quit)
		}
		return m, nil

	case spinner.TickMsg:
		if m.phase == "complete" || m.err != nil {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		var cmd tea.Cmd
		var progModel tea.Model
		progModel, cmd = m.prog.Update(msg)
		m.prog = progModel.(progress.Model)
		return m, cmd

	case eventMsg:
		evt := indexer.ProgressEvent(msg)
		m.phase = evt.Phase
		m.filesFound = evt.FilesFound
		m.filesIndexed = evt.FilesIndexed
		m.filesSkipped = evt.FilesSkipped
		m.filesError = evt.FilesError
		m.totalChunks = evt.TotalChunks
		m.elapsed = evt.Elapsed
		m.err = evt.Error

		if evt.Phase == "complete" {
			return m, tea.Sequence(tea.Println(m.finalView()), tea.Quit)
		}

		return m, waitForEvent(m.sub)
	}

	return m, nil
}

func (m model) View() string {
	switch m.phase {
	case "walk":
		return "  " + m.spinner.View() + " Walking repository...\n"
	case "index":
		return m.indexView()
	case "graph":
		return "  " + m.spinner.View() + " Building dependency graph...\n"
	case "complete":
		return ""
	default:
		return "  " + m.spinner.View() + " Starting...\n"
	}
}

func (m model) indexView() string {
	pct := 0.0
	total := m.filesIndexed + m.filesSkipped
	if m.filesFound > 0 {
		pct = float64(total) / float64(m.filesFound)
		if pct > 1.0 {
			pct = 1.0
		}
	}

	// All files skipped — nothing new to index
	if m.filesFound > 0 && total == m.filesFound && m.filesIndexed == 0 {
		return fmt.Sprintf("  %s %s\n",
			m.spinner.View(),
			InfoStyle.Render("All files up to date, checking graph..."))
	}

	bar := m.prog.ViewAs(pct)
	eta := m.eta()

	label := TitleStyle.Render(fmt.Sprintf("%d / %d", total, m.filesFound))
	errors := ""
	if m.filesError > 0 {
		errors = " " + ErrorStyle.Render(fmt.Sprintf("%d errs", m.filesError))
	}
	timer := ""
	if m.elapsed > 0 {
		timer = MutedStyle.Render("elapsed " + durationStr(m.elapsed))
	}
	etaStr := ""
	if eta > 0 && pct < 1.0 {
		etaStr = " " + HighlightStyle.Render("ETA "+durationStr(eta))
	}

	return fmt.Sprintf("%s%s %s%s%s\n%s\n%s%s\n",
		m.spinner.View(),
		SubtitleStyle.Render("Indexing"),
		label, etaStr, errors,
		bar,
		timer, etaStr)
}

func (m model) finalView() string {
	if m.err != nil {
		return fmt.Sprintf("%s %s\n",
			ErrorStyle.Render("✗ Indexing failed:"),
			InfoStyle.Render(m.err.Error()))
	}

	return fmt.Sprintf("%s\n  %s %s  %s %s  %s %s  %s %s  %s\n",
		SuccessStyle.Render("✓ Indexing complete!"),
		TitleStyle.Render("indexed"), InfoStyle.Render(fmt.Sprintf("%d", m.filesIndexed)),
		TitleStyle.Render("chunks"), InfoStyle.Render(fmt.Sprintf("%d", m.totalChunks)),
		TitleStyle.Render("skipped"), MutedStyle.Render(fmt.Sprintf("%d", m.filesSkipped)),
		TitleStyle.Render("errors"), m.errorCountStyle(),
		MutedStyle.Render("["+durationStr(m.elapsed)+"]"))
}

func (m model) eta() time.Duration {
	total := m.filesIndexed + m.filesSkipped
	if m.filesIndexed == 0 || m.filesFound == 0 || total >= m.filesFound {
		return 0
	}
	ratio := float64(total) / float64(m.filesFound)
	if ratio <= 0 {
		return 0
	}
	totalTime := time.Duration(float64(m.elapsed) / ratio)
	return totalTime - m.elapsed
}

func (m model) errorCountStyle() string {
	if m.filesError == 0 {
		return SuccessStyle.Render("0")
	}
	return ErrorStyle.Render(fmt.Sprintf("%d", m.filesError))
}

func durationStr(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
