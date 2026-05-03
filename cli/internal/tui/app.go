package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edrowsluo/new-mli/cli/internal/api"
	"github.com/edrowsluo/new-mli/cli/internal/config"
	"github.com/edrowsluo/new-mli/cli/internal/state"
)

// Screen identifiers.
const (
	screenLogin     = "login"
	screenDashboard = "dashboard"
)

// Styles
var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5F87")).
			MarginBottom(1)

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

// App is the top-level Bubble Tea model.
type App struct {
	screen string
	width  int
	height int

	cfg     *config.C
	http    *api.HTTPClient
	ws      *api.WSClient
	game    *state.GameState
	cancel  context.CancelFunc

	login     loginModel
	dashboard dashboardModel
}

// NewApp builds the initial app model.
func NewApp(cfg *config.C) *App {
	httpCli := api.NewHTTPClient(cfg.ServerURL)
	game := state.NewGameState()

	app := &App{
		screen: screenLogin,
		cfg:    cfg,
		http:   httpCli,
		ws:     &api.WSClient{},
		game:   game,
		login:  newLoginModel(),
	}
	app.dashboard = newDashboardModel(app)
	return app
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// If we have a saved token, try to validate it immediately.
	if a.cfg.Token != "" {
		return a.tryAutoLogin()
	}
	return textinput.Blink
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return a, tea.Quit
		}
	}

	switch a.screen {
	case screenLogin:
		return a.updateLogin(msg)
	case screenDashboard:
		return a.updateDashboard(msg)
	}

	return a, nil
}

// View implements tea.Model.
func (a *App) View() string {
	switch a.screen {
	case screenLogin:
		return a.loginView()
	case screenDashboard:
		return a.dashboardView()
	}
	return "unknown screen"
}

// --- Messages ---

type autoLoginResult struct {
	user *api.MeResponse
	err  error
}

func (a *App) tryAutoLogin() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5)
		defer cancel()
		user, err := a.http.WithToken(a.cfg.Token).Me(ctx)
		return autoLoginResult{user: user, err: err}
	}
}

func (a *App) switchScreen(name string) {
	a.screen = name
}
