package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	width = 96
)

type httpResp struct {
	status int
}

type config struct {
	Title        string        `toml:"title"`
	Applications []application `toml:"applications"`
}

type application struct {
	Name        string
	URL         string
	Description string
	httpResp    httpResp
}

type metadata struct {
	title  string
	status string
}

type model struct {
	applications []application
	cursor       int
	metadata     metadata

	client *http.Client
}

func (m model) GetAppURLs() []string {
	var urls []string
	for _, v := range m.applications {
		urls = append(urls, v.URL)
	}

	return urls
}

func (m model) Init() tea.Cmd {
	return m.checkServers()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:

		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.applications)-1 {
				m.cursor++
			}

		case "ctrl+h":
			return m, m.checkServers()
		}

	case statusMsg:
		m.metadata.status = "Looking good..."
		for i, app := range m.applications {
			m.applications[i].httpResp.status = msg[app.URL]
			if m.applications[i].httpResp.status != http.StatusOK {
				m.metadata.status = fmt.Sprintf("%s might be having issues...", app.Name)
			}
		}
		return m, m.checkServers()
	}

	return m, nil
}

func (m model) View() string {
	ui := strings.Builder{}

	// ==========================================================================
	// title
	{
		title := titleStyle.
			Background(lipgloss.Color("12")).
			Render(m.metadata.title)

		ui.WriteString(title + "\n")
	}

	var items []string
	for i, app := range m.applications {
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">"
		}

		s := fmt.Sprintf(
			"%s %s status: %d\n%s",
			cursor,
			app.Name,
			app.httpResp.status,
			url(app.URL),
		)

		// if app.httpResp.status == 0 {
		// 	s = fmt.Sprintf(
		// 		"%s %s status: \n%s\n\n",
		// 		cursor,
		// 		app.Name,
		// 		url(app.URL),
		// 	)
		// }

		items = append(items, listItem(s))
	}

	apps := lipgloss.JoinVertical(
		lipgloss.Top,
		items...,
	)

	ui.WriteString(apps)

	ui.WriteString("\n")

	{
		w := lipgloss.Width

		statusKey := statusStyle.Render("STATUS")
		encoding := encodingStyle.Render(time.Now().Format(time.UnixDate))
		statusVal := statusText.Copy().
			Width(width - w(statusKey) - w(encoding)).
			Render(m.metadata.status)

		bar := lipgloss.JoinHorizontal(
			lipgloss.Top,
			statusKey,
			statusVal,
			encoding,
		)

		ui.WriteString(statusBarStyle.Width(width).Render(bar))
	}

	return docStyle.Render(ui.String())
}

func loadConfigFile() (config, error) {
	f := ".homie.toml"
	if _, err := os.Stat(f); err != nil {
		return config{}, err
	}

	var cfg config

	_, err := toml.DecodeFile(f, &cfg)
	if err != nil {
		return config{}, err
	}
	return cfg, nil
}

type statusMsg map[string]int

func (m model) checkServers() tea.Cmd {
	return tea.Tick(time.Second*30, func(t time.Time) tea.Msg {
		msg := make(statusMsg)

		for _, app := range m.applications {
			log.Println("checking: ", app.URL)
			res, err := m.client.Get(app.URL)
			if err != nil {
				msg[app.URL] = 0
				continue
			}
			log.Println("res: ", res.StatusCode)
			msg[app.URL] = res.StatusCode
		}
		return msg
	})
}

func main() {
	// ====================================================================
	// load config file
	cfg, err := loadConfigFile()
	if err != nil {
		log.Fatal(err)
	}

	// ====================================================================
	// clients
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	initialModel := model{
		applications: cfg.Applications,
		metadata: metadata{
			title:  cfg.Title,
			status: "loading...",
		},
		client: httpClient,
	}

	log.Println("starting homie...")

	p := tea.NewProgram(initialModel)
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
