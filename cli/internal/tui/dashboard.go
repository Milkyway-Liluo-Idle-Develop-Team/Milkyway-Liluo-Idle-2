package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pb "github.com/edrowsluo/new-mli/backend/pb"

	"github.com/edrowsluo/new-mli/cli/internal/state"
)

// dashboardModel is the main game screen.
type dashboardModel struct {
	app       *App
	cmdInput  textinput.Model
	logView   viewport.Model
	ready     bool
}

func newDashboardModel(app *App) dashboardModel {
	m := dashboardModel{app: app}

	m.cmdInput = textinput.New()
	m.cmdInput.Placeholder = "type a command..."
	m.cmdInput.Prompt = "> "
	m.cmdInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
	m.cmdInput.CharLimit = 256

	m.logView = viewport.New(80, 10)
	m.logView.SetContent("")

	return m
}

// wsMsg is delivered by the WS read goroutine.
type wsMsg struct{ env *pb.Envelope }

func wsListenCmd(recv <-chan *pb.Envelope) tea.Cmd {
	return func() tea.Msg {
		env, ok := <-recv
		if !ok {
			return wsMsg{env: &pb.Envelope{Type: "__closed"}}
		}
		return wsMsg{env: env}
	}
}

func (a *App) updateDashboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if !a.dashboard.ready {
			a.dashboard.logView = viewport.New(a.width-4, a.height/3)
			a.dashboard.logView.SetContent("")
			a.dashboard.ready = true
		} else {
			a.dashboard.logView.Width = a.width - 4
			a.dashboard.logView.Height = a.height / 3
		}
		return a, nil

	case wsMsg:
		// Process server message and schedule next read.
		a.handleWS(msg.env)
		return a, wsListenCmd(a.ws.Recv())

	case tea.KeyMsg:
		// Global shortcuts.
		switch msg.Type {
		case tea.KeyEsc:
			// Blur command input.
			a.dashboard.cmdInput.Blur()
			return a, nil
		}

		// If command input is focused, handle typing.
		if a.dashboard.cmdInput.Focused() {
			switch msg.Type {
			case tea.KeyEnter:
				cmd := strings.TrimSpace(a.dashboard.cmdInput.Value())
				a.dashboard.cmdInput.SetValue("")
				if cmd != "" {
					a.execCommand(cmd)
				}
				return a, nil
			case tea.KeyCtrlC:
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.dashboard.cmdInput, cmd = a.dashboard.cmdInput.Update(msg)
			return a, cmd
		}

		// When input is blurred, keys control navigation.
		switch msg.String() {
		case ":", "/":
			a.dashboard.cmdInput.Focus()
			return a, textinput.Blink
		case "q":
			return a, tea.Quit
		}
	}

	// Pass to log viewport (for scrolling).
	var cmd tea.Cmd
	a.dashboard.logView, cmd = a.dashboard.logView.Update(msg)
	return a, cmd
}

func (a *App) dashboardView() string {
	if !a.dashboard.ready {
		return "Loading..."
	}

	// Header
	header := a.renderHeader()

	// Three-column panels: Skills | Event Queue | Equipment
	cols := a.renderPanels()

	// Inventory strip
	inv := a.renderInventory()

	// Command input
	cmdBar := a.dashboard.cmdInput.View()

	// Log
	logPanel := a.dashboard.logView.View()

	// Assemble
	content := strings.Join([]string{
		header,
		cols,
		inv,
		cmdBar,
		logPanel,
	}, "\n")

	return appStyle.Render(content)
}

func (a *App) renderHeader() string {
	wsStatus := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("●")
	if a.ws == nil {
		wsStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("●")
	}
	return lipgloss.NewStyle().
		Bold(true).
		Render(fmt.Sprintf("User: %s %s connected", a.game.Username, wsStatus))
}

func (a *App) renderPanels() string {
	// Skills panel
	var skillsContent strings.Builder
	skillsContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Skills") + "\n")
	for id, slot := range a.game.Skills {
		skillsContent.WriteString(fmt.Sprintf("ID:%d Lv.%.0f XP:%.0f\n", id, slot.Level, slot.XP))
	}
	if len(a.game.Skills) == 0 {
		skillsContent.WriteString("(no skills yet)")
	}

	// Events panel
	var eventsContent strings.Builder
	eventsContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Event Queue") + "\n")
	for qid, entries := range a.game.EventQueues {
		eventsContent.WriteString(fmt.Sprintf("Queue %d:\n", qid))
		for _, e := range entries {
			eventsContent.WriteString(fmt.Sprintf("  E:%d P:%.1f\n", e.EventID, e.Progress))
		}
	}
	if len(a.game.EventQueues) == 0 {
		eventsContent.WriteString("(no active events)")
	}

	// Equipment panel
	var equipContent strings.Builder
	equipContent.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Equipment") + "\n")
	for slot, item := range a.game.Equipment {
		equipContent.WriteString(fmt.Sprintf("%s: %d\n", slot, item.ID))
	}
	if len(a.game.Equipment) == 0 {
		equipContent.WriteString("(nothing equipped)")
	}

	// Styles
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(0, 1)

	w := (a.width - 8) / 3
	if w < 20 {
		w = 20
	}

	skillsPanel := panelStyle.Width(w).Render(skillsContent.String())
	eventsPanel := panelStyle.Width(w).Render(eventsContent.String())
	equipPanel := panelStyle.Width(w).Render(equipContent.String())

	return lipgloss.JoinHorizontal(lipgloss.Top, skillsPanel, eventsPanel, equipPanel)
}

func (a *App) renderInventory() string {
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(0, 1).
		Width(a.width - 4)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render("Inventory") + "\n")
	count := 0
	for key, qty := range a.game.Inventory {
		b.WriteString(fmt.Sprintf("%d:%d=%.1f  ", key.ID, key.State, qty))
		count++
		if count%4 == 0 {
			b.WriteString("\n")
		}
	}
	if len(a.game.Inventory) == 0 {
		b.WriteString("(empty)")
	}
	return panelStyle.Render(b.String())
}

// handleWS processes a single server envelope.
func (a *App) handleWS(env *pb.Envelope) {
	switch env.Type {
	case "state.diff":
		diff, err := state.DecodeStateDiff(env)
		if err != nil {
			a.game.Logf("err", "decode diff: %v", err)
			return
		}
		a.game.ApplyDiff(diff)
		a.refreshLog()

	case "inventory.equip.ok", "inventory.unequip.ok":
		resp, err := state.DecodeEquipResponse(env)
		if err != nil {
			a.game.Logf("err", "decode equip resp: %v", err)
			return
		}
		// Sync equipment from server response.
		a.game.Equipment = make(map[string]*state.EquippedItem)
		for slot, it := range resp.Equipped {
			a.game.Equipment[slot] = &state.EquippedItem{ID: it.ItemId, State: it.ItemState}
		}
		a.game.Logf("info", "%s ok", env.Type)
		a.refreshLog()

	case "inventory.equip.err", "inventory.unequip.err":
		if env.Error != nil {
			a.game.Logf("err", "%s: %s", env.Type, env.Error.Message)
		}
		a.refreshLog()

	case "ping.ok":
		// Keepalive response — no need to log every time.

	case "__error":
		a.game.Logf("err", "ws error: %s", env.Error.Message)
		a.refreshLog()

	case "__closed":
		a.game.Logf("warn", "connection closed")
		a.refreshLog()

	default:
		a.game.Logf("info", "recv: %s", env.Type)
		a.refreshLog()
	}
}

// refreshLog rebuilds the log viewport content.
func (a *App) refreshLog() {
	var lines []string
	for _, entry := range a.game.Log {
		lines = append(lines, fmt.Sprintf("[%02d:%02d] %s", entry.Time.Minute(), entry.Time.Second(), entry.Message))
	}
	a.dashboard.logView.SetContent(strings.Join(lines, "\n"))
	a.dashboard.logView.GotoBottom()
}

// execCommand parses and executes a user-typed command.
func (a *App) execCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "equip":
		if len(parts) != 3 {
			a.game.Logf("err", "usage: equip <item_id> <slot>")
			return
		}
		var itemID int32
		var slot string
		fmt.Sscanf(parts[1], "%d", &itemID)
		slot = parts[2]
		req := &pb.EquipReq{ItemId: itemID, Slot: slot}
		if err := a.ws.Send("", "inventory.equip", req); err != nil {
			a.game.Logf("err", "send equip: %v", err)
		}

	case "unequip":
		if len(parts) != 2 {
			a.game.Logf("err", "usage: unequip <slot>")
			return
		}
		req := &pb.UnequipReq{Slot: parts[1]}
		if err := a.ws.Send("", "inventory.unequip", req); err != nil {
			a.game.Logf("err", "send unequip: %v", err)
		}

	case "ping":
		if err := a.ws.Send("", "ping", nil); err != nil {
			a.game.Logf("err", "send ping: %v", err)
		}

	case "help":
		a.game.Logf("info", "commands: equip <item_id> <slot>, unequip <slot>, ping, help")

	default:
		a.game.Logf("err", "unknown command: %s", parts[0])
	}
	a.refreshLog()
}
