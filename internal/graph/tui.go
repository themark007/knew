package graph

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Lipgloss styles ─────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED")).
			Padding(0, 1)

	podStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4FC3F7"))

	svcStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#81C784"))

	ingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFB74D"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Italic(true)

	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F59E0B"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4B5563"))
)

// ─── list.Item adapter ───────────────────────────────────────────────────────

type nodeItem struct {
	Node
}

func (n nodeItem) Title() string {
	icon := nodeTypeIcon(n.Type)
	style := nodeTypeStyle(n.Type)
	return style.Render(fmt.Sprintf("%s %s/%s", icon, n.Namespace, n.Name))
}

func (n nodeItem) Description() string {
	return fmt.Sprintf("[%s]", strings.ToUpper(string(n.Type)))
}

func (n nodeItem) FilterValue() string {
	return n.Namespace + "/" + n.Name
}

func nodeTypeIcon(t NodeType) string {
	switch t {
	case NodePod:
		return "●"
	case NodeService:
		return "⚙"
	case NodeIngress:
		return "⮕"
	default:
		return "○"
	}
}

func nodeTypeStyle(t NodeType) lipgloss.Style {
	switch t {
	case NodePod:
		return podStyle
	case NodeService:
		return svcStyle
	case NodeIngress:
		return ingStyle
	default:
		return lipgloss.NewStyle()
	}
}

// ─── TUI model ───────────────────────────────────────────────────────────────

type TUIModel struct {
	graph    *Graph
	list     list.Model
	detail   viewport.Model
	width    int
	height   int
	ready    bool
	quitting bool
}

// NewTUIModel creates an interactive bubbletea graph explorer.
func NewTUIModel(g *Graph) TUIModel {
	items := make([]list.Item, len(g.Nodes))
	for i, n := range g.Nodes {
		items[i] = nodeItem{n}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	l := list.New(items, delegate, 0, 0)
	l.Title = "knet graph"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	vp := viewport.New(0, 0)

	return TUIModel{
		graph:  g,
		list:   l,
		detail: vp,
	}
}

func (m TUIModel) Init() tea.Cmd {
	return nil
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listW := m.width / 2
		detailW := m.width - listW - 2
		if detailW < 20 {
			detailW = 20
		}
		m.list.SetSize(listW, m.height-3)
		m.detail = viewport.New(detailW, m.height-3)
		m.ready = true
		m.detail.SetContent(m.buildDetail())

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter", " ":
			m.detail.SetContent(m.buildDetail())
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	m.detail.SetContent(m.buildDetail())
	m.detail, cmd = m.detail.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m TUIModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	help := helpStyle.Render("↑/↓ navigate  /  filter  q  quit  enter  details")

	left := borderStyle.Width(m.width/2 - 2).Render(m.list.View())
	right := borderStyle.Width(m.width - m.width/2 - 4).Render(m.detail.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return body + "\n" + help
}

func (m TUIModel) buildDetail() string {
	item, ok := m.list.SelectedItem().(nodeItem)
	if !ok {
		return "Select a node to see details"
	}
	n := item.Node
	var b strings.Builder

	b.WriteString(detailTitleStyle.Render(fmt.Sprintf("%s %s/%s", nodeTypeIcon(n.Type), n.Namespace, n.Name)))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Type:  %s\n", strings.ToUpper(string(n.Type))))
	if n.Extra != "" {
		b.WriteString("\n" + n.Extra + "\n")
	}
	if len(n.Labels) > 0 {
		b.WriteString("\nLabels:\n")
		for k, v := range n.Labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
	}

	outs := m.graph.OutNeighbours(n.ID)
	if len(outs) > 0 {
		b.WriteString("\nConnects to:\n")
		for _, o := range outs {
			style := nodeTypeStyle(o.Type)
			b.WriteString(fmt.Sprintf("  %s %s\n", nodeTypeIcon(o.Type), style.Render(o.Namespace+"/"+o.Name)))
		}
	}

	ins := m.graph.InNeighbours(n.ID)
	if len(ins) > 0 {
		b.WriteString("\nReceives from:\n")
		for _, i := range ins {
			style := nodeTypeStyle(i.Type)
			b.WriteString(fmt.Sprintf("  %s %s\n", nodeTypeIcon(i.Type), style.Render(i.Namespace+"/"+i.Name)))
		}
	}

	edges := m.graph.OutEdgesFrom(n.ID)
	if len(edges) > 0 {
		b.WriteString("\nEdge details:\n")
		for _, e := range edges {
			target := m.graph.NodeByID(e.To)
			if target == nil {
				continue
			}
			b.WriteString(fmt.Sprintf("  ──[%s]──► %s/%s\n", e.Type, target.Namespace, target.Name))
		}
	}

	return b.String()
}

// RunTUI launches the interactive graph TUI.
func RunTUI(g *Graph) error {
	m := NewTUIModel(g)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
