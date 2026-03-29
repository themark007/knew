package cmd

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/themark007/knew/internal/k8s"
	"github.com/themark007/knew/internal/output"
)

var watchInterval int

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Live auto-refreshing network scan",
	Long: `Continuously scan and display the cluster network topology,
refreshing at a configurable interval.

Controls:
  q / Ctrl+C   quit
  p            pause/resume
  r            force refresh now

Examples:
  knet watch                   # refresh every 5 seconds
  knet watch -n production     # watch production namespace
  knet watch --interval 10     # refresh every 10 seconds
  knet watch -A                # all namespaces`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cs, defaultNS, err := k8s.NewClient(kubeconfig, kubeContext)
		if err != nil {
			return fmt.Errorf("connecting to cluster: %w", err)
		}
		ns := namespace
		if ns == "" {
			ns = defaultNS
		}

		m := newWatchModel(k8s.BuildOptions{
			Clientset:       cs,
			Namespace:       ns,
			AllNamespaces:   allNs,
			LabelSelector:   labelSel,
			Timeout:         timeout,
			IncludePolicies: true,
		}, time.Duration(watchInterval)*time.Second)

		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
	watchCmd.Flags().IntVar(&watchInterval, "interval", 5, "refresh interval in seconds")
}

// ─── Watch TUI model ─────────────────────────────────────────────────────────

type watchState int

const (
	watchRunning watchState = iota
	watchPaused
)

type tickMsg time.Time
type fetchDoneMsg struct {
	topo *k8s.Topology
	err  error
}

type watchModel struct {
	opts      k8s.BuildOptions
	interval  time.Duration
	topo      *k8s.Topology
	err       error
	state     watchState
	width     int
	height    int
	lastFetch time.Time
	fetching  bool
}

func newWatchModel(opts k8s.BuildOptions, interval time.Duration) watchModel {
	return watchModel{opts: opts, interval: interval, state: watchRunning}
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(fetchTopo(m.opts), tick(m.interval))
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		if m.state == watchRunning && !m.fetching {
			m.fetching = true
			return m, tea.Batch(fetchTopo(m.opts), tick(m.interval))
		}
		return m, tick(m.interval)

	case fetchDoneMsg:
		m.fetching = false
		m.lastFetch = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.topo = msg.topo
			m.err = nil
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "p":
			if m.state == watchRunning {
				m.state = watchPaused
			} else {
				m.state = watchRunning
			}
		case "r":
			if !m.fetching {
				m.fetching = true
				return m, fetchTopo(m.opts)
			}
		}
	}
	return m, nil
}

var (
	watchTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	watchHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	watchErrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
)

func (m watchModel) View() string {
	stateStr := "● LIVE"
	if m.state == watchPaused {
		stateStr = "⏸ PAUSED"
	}
	if m.fetching {
		stateStr = "⟳ FETCHING"
	}

	title := watchTitleStyle.Render(fmt.Sprintf("  knet watch  %s", stateStr))
	ts := ""
	if !m.lastFetch.IsZero() {
		ts = fmt.Sprintf("  Last refresh: %s", m.lastFetch.Format("15:04:05"))
	}
	help := watchHelpStyle.Render("  q quit  p pause/resume  r refresh now")

	header := title + "\n" + ts + "\n" + help + "\n"
	header += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#334155")).Render(repeat("─", m.width-2)) + "\n\n"

	if m.err != nil {
		return header + watchErrStyle.Render("  Error: "+m.err.Error())
	}
	if m.topo == nil {
		return header + "  Loading..."
	}

	// Capture table output into string
	var buf captureWriter
	output.PrintPods(&buf, m.topo.Pods, false)
	pods := buf.String()
	buf.Reset()
	output.PrintServices(&buf, m.topo.Services, false)
	svcs := buf.String()

	summary := fmt.Sprintf("  pods=%d  services=%d  policies=%d  ingresses=%d",
		len(m.topo.Pods), len(m.topo.Services), len(m.topo.Policies), len(m.topo.Ingresses))

	return header + summary + "\n\n" +
		"  PODS\n" + pods + "\n" +
		"  SERVICES\n" + svcs
}

func fetchTopo(opts k8s.BuildOptions) tea.Cmd {
	return func() tea.Msg {
		topo, err := k8s.BuildTopology(opts)
		return fetchDoneMsg{topo: topo, err: err}
	}
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// captureWriter is an in-memory io.Writer.
type captureWriter struct {
	buf []byte
}

func (c *captureWriter) Write(p []byte) (int, error) {
	c.buf = append(c.buf, p...)
	return len(p), nil
}

func (c *captureWriter) String() string { return string(c.buf) }
func (c *captureWriter) Reset()         { c.buf = nil }

// Ensure captureWriter satisfies io.Writer (used in output functions)
var _ = (*captureWriter)(nil)

// Keep os import used
var _ = os.Stderr
