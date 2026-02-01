package main

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const maxPlanErrorsInTUI = 20

// planMsg is a message from the plan generator or log tee.
type planMsg struct {
	LogErr     string
	Done       bool
	Err        error
	TrackCount int
	PlanPath   string
}

// planModel is the Bubble Tea model for the plan TUI.
type planModel struct {
	phase     string
	errors    []string
	done      bool
	cancelling bool
	err       error
	tracks    int
	planPath  string
	logPath   string
	ch        chan planMsg
	cancel    context.CancelFunc
	width     int
	height    int
}

func newPlanModel(logPath string, ch chan planMsg, cancel context.CancelFunc) *planModel {
	return &planModel{
		phase:   "Generating plan...",
		errors:  make([]string, 0, maxPlanErrorsInTUI),
		logPath: logPath,
		ch:      ch,
		cancel:  cancel,
	}
}

func (m *planModel) Init() tea.Cmd {
	return m.waitForMsg()
}

func (m *planModel) waitForMsg() tea.Cmd {
	return func() tea.Msg {
		return <-m.ch
	}
}

func (m *planModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, m.waitForMsg()
	case tea.KeyMsg:
		key := msg.String()
		if key == "q" || key == "ctrl+c" {
			if m.done {
				return m, tea.Quit
			}
			if !m.cancelling && m.cancel != nil {
				m.cancel()
				m.cancelling = true
			}
		}
		return m, m.waitForMsg()
	case planMsg:
		if msg.LogErr != "" {
			m.errors = append(m.errors, msg.LogErr)
			if len(m.errors) > maxPlanErrorsInTUI {
				m.errors = m.errors[len(m.errors)-maxPlanErrorsInTUI:]
			}
			return m, m.waitForMsg()
		}
		if msg.Done {
			m.done = true
			m.err = msg.Err
			m.tracks = msg.TrackCount
			m.planPath = msg.PlanPath
			return m, tea.Quit
		}
		return m, m.waitForMsg()
	default:
		return m, m.waitForMsg()
	}
}

func (m *planModel) View() string {
	var b strings.Builder
	b.WriteString("  musicdl plan\n\n")
	b.WriteString("  " + m.phase + "\n")
	b.WriteString("  Log file: " + m.logPath + "\n\n")
	if m.done {
		if m.cancelling {
			b.WriteString("  Stopping...\n")
		}
		if m.err != nil {
			b.WriteString("  Error: " + m.err.Error() + "\n")
		} else if !m.cancelling {
			b.WriteString("  Plan generated successfully\n")
			b.WriteString(fmt.Sprintf("  Total tracks: %d\n", m.tracks))
			if m.planPath != "" {
				b.WriteString("  Plan file: " + m.planPath + "\n")
			}
		}
	}
	if len(m.errors) > 0 {
		b.WriteString("\n  Recent errors / warnings:\n")
		start := 0
		if len(m.errors) > 10 {
			start = len(m.errors) - 10
		}
		for i := start; i < len(m.errors); i++ {
			b.WriteString("    • " + truncatePlanErr(m.errors[i], 70) + "\n")
		}
	}
	b.WriteString("\n  q: quit  (Ctrl+C: stop)\n")
	return b.String()
}

func truncatePlanErr(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// RunPlanTUI runs the TUI for plan. The caller must run the generator in a goroutine
// and send a planMsg with Done=true (and Err, TrackCount, PlanPath) when finished.
// cancel is called when the user presses q or Ctrl+C to stop mid-run; may be nil.
// Log errors can be sent to logErrCh (optional). Returns the final error from the model.
func RunPlanTUI(logPath string, planCh chan planMsg, logErrCh <-chan string, cancel context.CancelFunc) error {
	model := newPlanModel(logPath, planCh, cancel)
	if logErrCh != nil {
		go func() {
			for s := range logErrCh {
				planCh <- planMsg{LogErr: s}
			}
		}()
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	pm, ok := finalModel.(*planModel)
	if !ok {
		return nil
	}
	return pm.err
}
