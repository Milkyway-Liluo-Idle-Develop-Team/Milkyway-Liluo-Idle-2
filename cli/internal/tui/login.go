package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// loginModel is the login / register form.
type loginModel struct {
	focusIndex int
	inputs     []textinput.Model
	errMsg     string
	status     string
}

func newLoginModel() loginModel {
	m := loginModel{
		inputs: make([]textinput.Model, 2),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
		t.CharLimit = 32

		switch i {
		case 0:
			t.Placeholder = "Username"
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "Password"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
			t.PromptStyle = noStyle
			t.TextStyle = noStyle
		}
		m.inputs[i] = t
	}
	return m
}

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
	noStyle      = lipgloss.NewStyle()
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

func (a *App) updateLogin(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case autoLoginResult:
		if msg.err != nil {
			a.cfg.Token = ""
			a.login.errMsg = "auto-login failed: " + msg.err.Error()
			return a, nil
		}
		a.game.Username = msg.user.Username
		a.game.UserID = msg.user.ID
		return a, a.enterDashboard()

	case loginResultMsg:
		if msg.err != nil {
			a.login.errMsg = msg.err.Error()
			return a, nil
		}
		a.cfg.Token = msg.token
		a.game.Username = msg.username
		a.game.UserID = msg.userID
		return a, a.enterDashboard()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyTab, tea.KeyShiftTab, tea.KeyEnter, tea.KeyUp, tea.KeyDown:
			s := msg.String()
			if s == "enter" && a.login.focusIndex == len(a.login.inputs) {
				return a, a.doLogin()
			}
			if s == "enter" && a.login.focusIndex == len(a.login.inputs)+1 {
				return a, a.doRegister()
			}

			if s == "up" || s == "shift+tab" {
				a.login.focusIndex--
			} else {
				a.login.focusIndex++
			}

			if a.login.focusIndex > len(a.login.inputs)+1 {
				a.login.focusIndex = 0
			} else if a.login.focusIndex < 0 {
				a.login.focusIndex = len(a.login.inputs) + 1
			}

			cmds := make([]tea.Cmd, len(a.login.inputs))
			for i := range a.login.inputs {
				if i == a.login.focusIndex {
					cmds[i] = a.login.inputs[i].Focus()
					a.login.inputs[i].PromptStyle = focusedStyle
					a.login.inputs[i].TextStyle = focusedStyle
					continue
				}
				a.login.inputs[i].Blur()
				a.login.inputs[i].PromptStyle = noStyle
				a.login.inputs[i].TextStyle = noStyle
			}
			return a, tea.Batch(cmds...)
		}
	}

	// Pass messages to focused input.
	cmd := a.updateLoginInputs(msg)
	return a, cmd
}

func (a *App) updateLoginInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(a.login.inputs))
	for i := range a.login.inputs {
		var m textinput.Model
		m, cmds[i] = a.login.inputs[i].Update(msg)
		a.login.inputs[i] = m
	}
	return tea.Batch(cmds...)
}

func (a *App) loginView() string {
	var b string
	b += titleStyle.Render("⚔  MLI - Command Line Client") + "\n\n"

	for i := range a.login.inputs {
		b += a.login.inputs[i].View() + "\n"
	}

	buttonLogin := blurredStyle.Render("[ Login ]")
	buttonRegister := blurredStyle.Render("[ Register ]")
	if a.login.focusIndex == len(a.login.inputs) {
		buttonLogin = focusedStyle.Render("[ Login ]")
	}
	if a.login.focusIndex == len(a.login.inputs)+1 {
		buttonRegister = focusedStyle.Render("[ Register ]")
	}
	b += "\n" + buttonLogin + "  " + buttonRegister + "\n"

	if a.login.errMsg != "" {
		b += "\n" + errStyle.Render(a.login.errMsg) + "\n"
	}
	if a.login.status != "" {
		b += "\n" + infoStyle.Render(a.login.status) + "\n"
	}

	b += "\n" + infoStyle.Render("Tab/Shift+Tab to navigate • Enter to submit • Ctrl+C to quit") + "\n"

	return appStyle.Render(b)
}

// --- Commands ---

type loginResultMsg struct {
	token    string
	username string
	userID   int64
	err      error
}

func (a *App) doLogin() tea.Cmd {
	return func() tea.Msg {
		username := a.login.inputs[0].Value()
		password := a.login.inputs[1].Value()
		ctx, cancel := context.WithTimeout(context.Background(), 10)
		defer cancel()
		resp, err := a.http.Login(ctx, username, password)
		if err != nil {
			return loginResultMsg{err: err}
		}
		return loginResultMsg{
			token:    resp.Session.Token,
			username: resp.User.Username,
			userID:   resp.User.ID,
		}
	}
}

func (a *App) doRegister() tea.Cmd {
	return func() tea.Msg {
		username := a.login.inputs[0].Value()
		password := a.login.inputs[1].Value()
		ctx, cancel := context.WithTimeout(context.Background(), 10)
		defer cancel()
		resp, err := a.http.Register(ctx, username, password)
		if err != nil {
			return loginResultMsg{err: err}
		}
		return loginResultMsg{
			token:    resp.Session.Token,
			username: resp.User.Username,
			userID:   resp.User.ID,
		}
	}
}

func (a *App) enterDashboard() tea.Cmd {
	return func() tea.Msg {
		a.switchScreen(screenDashboard)
		// Save token.
		_ = a.cfg.Save()

		// Fetch game config.
		ctx, cancel := context.WithTimeout(context.Background(), 10)
		defer cancel()
		gc, err := a.http.WithToken(a.cfg.Token).FetchGameConfig(ctx)
		if err != nil {
			a.game.Logf("warn", "fetch config: %v", err)
		} else {
			// Store raw bytes; dashboard can parse as needed.
			// For now just log success.
			a.game.Logf("info", "game config loaded")
		}
		_ = gc

		// Connect WebSocket.
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10)
		defer cancel2()
		if err := a.ws.Connect(ctx2, a.cfg.ServerURL, a.cfg.Token); err != nil {
			a.game.Logf("err", "ws connect: %v", err)
			return nil
		}
		a.game.Logf("info", "connected to server")

		return nil
	}
}
