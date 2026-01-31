package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sv4u/musicdl/download/plan"
)

const maxErrorsInTUI = 20

// downloadMsg is a message from the download executor or log tee.
type downloadMsg struct {
	Item   *plan.PlanItem
	Stats  map[string]int
	Err    error
	LogErr string
}

// downloadModel is the Bubble Tea model for the download TUI.
type downloadModel struct {
	completed    int
	failed       int
	total        int
	currentTrack string
	errors       []string
	logPath      string
	done         bool
	execErr      error
	finalStats   map[string]int
	ch           chan downloadMsg
	width        int
	height       int
}

func newDownloadModel(logPath string, total int, ch chan downloadMsg) *downloadModel {
	return &downloadModel{
		total:   total,
		logPath: logPath,
		errors:  make([]string, 0, maxErrorsInTUI),
		ch:      ch,
	}
}

func (m *downloadModel) Init() tea.Cmd {
	return m.waitForMsg()
}

func (m *downloadModel) waitForMsg() tea.Cmd {
	return func() tea.Msg {
		return <-m.ch
	}
}

func (m *downloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, m.waitForMsg()
	case tea.KeyMsg:
		if m.done && msg.String() == "q" {
			return m, tea.Quit
		}
		return m, m.waitForMsg()
	case downloadMsg:
		if msg.LogErr != "" {
			m.errors = append(m.errors, msg.LogErr)
			if len(m.errors) > maxErrorsInTUI {
				m.errors = m.errors[len(m.errors)-maxErrorsInTUI:]
			}
			return m, m.waitForMsg()
		}
		if msg.Item != nil {
			status := msg.Item.GetStatus()
			switch status {
			case plan.PlanItemStatusCompleted:
				m.completed++
				m.currentTrack = ""
			case plan.PlanItemStatusFailed:
				m.failed++
				errStr := msg.Item.GetError()
				name := msg.Item.Name
				if name != "" {
					m.errors = append(m.errors, name+": "+errStr)
				} else {
					m.errors = append(m.errors, errStr)
				}
				if len(m.errors) > maxErrorsInTUI {
					m.errors = m.errors[len(m.errors)-maxErrorsInTUI:]
				}
				m.currentTrack = ""
			case plan.PlanItemStatusInProgress:
				m.currentTrack = msg.Item.Name
			}
			return m, m.waitForMsg()
		}
		if msg.Stats != nil {
			m.done = true
			m.execErr = msg.Err
			m.finalStats = msg.Stats
			if msg.Stats["completed"] != 0 || msg.Stats["failed"] != 0 || msg.Stats["total"] != 0 {
				m.completed = msg.Stats["completed"]
				m.failed = msg.Stats["failed"]
				m.total = msg.Stats["total"]
			}
			return m, tea.Quit
		}
		return m, m.waitForMsg()
	default:
		return m, m.waitForMsg()
	}
}

func (m *downloadModel) View() string {
	var b strings.Builder
	b.WriteString("  musicdl download\n\n")
	b.WriteString(fmt.Sprintf("  Completed: %d  Failed: %d  Total: %d\n", m.completed, m.failed, m.total))
	if m.currentTrack != "" {
		b.WriteString("  Current: " + truncate(m.currentTrack, 60) + "\n")
	}
	b.WriteString("  Log file: " + m.logPath + "\n\n")
	if len(m.errors) > 0 {
		b.WriteString("  Recent errors:\n")
		start := 0
		if len(m.errors) > 10 {
			start = len(m.errors) - 10
		}
		for i := start; i < len(m.errors); i++ {
			b.WriteString("    • " + truncate(m.errors[i], 70) + "\n")
		}
	}
	if m.done && m.execErr != nil {
		b.WriteString("\n  Fatal: " + m.execErr.Error() + "\n")
	}
	if m.done {
		b.WriteString("\n  Press q to quit.\n")
	}
	return b.String()
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// RunDownloadTUI runs the TUI for download. The caller must run the executor in a goroutine
// and send progress/done messages to progressCh. Log errors can be sent to logErrCh (optional);
// they are forwarded to progressCh. Returns final stats and executor error from the model after quit.
func RunDownloadTUI(
	logPath string,
	totalTracks int,
	progressCh chan downloadMsg,
	logErrCh <-chan string,
) (stats map[string]int, execErr error) {
	model := newDownloadModel(logPath, totalTracks, progressCh)
	if logErrCh != nil {
		go func() {
			for s := range logErrCh {
				progressCh <- downloadMsg{LogErr: s}
			}
		}()
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	dm, ok := finalModel.(*downloadModel)
	if !ok {
		return nil, nil
	}
	if dm.finalStats != nil {
		return dm.finalStats, dm.execErr
	}
	return map[string]int{"completed": dm.completed, "failed": dm.failed, "total": dm.total}, dm.execErr
}

func countPendingTracks(p *plan.DownloadPlan) int {
	n := 0
	for _, item := range p.Items {
		if item.ItemType == plan.PlanItemTypeTrack && item.Status == plan.PlanItemStatusPending {
			n++
		}
	}
	return n
}
