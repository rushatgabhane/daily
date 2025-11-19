package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Config struct {
	FormURL string `json:"form_url"`
}

var (
	focusedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	titleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	cursorStyle   = focusedStyle.Copy()
	noStyle       = lipgloss.NewStyle()
	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

type model struct {
	focusIndex  int
	inputs      []textinput.Model
	progressInput textinput.Model
	hoursInput  textinput.Model
	issueTitle  string
	err         error
	submitted   bool
	fetching    bool
	submitMsg   string
	formURL     string
}

func initialModel(formURL string) model {
	m := model{
		inputs:  make([]textinput.Model, 4),
		formURL: formURL,
	}

	// Email
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "your.email@example.com"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 100
	m.inputs[0].Width = 60
	m.inputs[0].Prompt = ""
	m.inputs[0].SetValue(getGitEmail())

	// Date
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "DD/MM/YYYY"
	m.inputs[1].CharLimit = 10
	m.inputs[1].Width = 60
	m.inputs[1].Prompt = ""
	m.inputs[1].SetValue(time.Now().Format("02/01/2006"))

	// Issue Link
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "https://github.com/..."
	m.inputs[2].CharLimit = 200
	m.inputs[2].Width = 60
	m.inputs[2].Prompt = ""

	// Project Name
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "N/A"
	m.inputs[3].CharLimit = 100
	m.inputs[3].Width = 60
	m.inputs[3].Prompt = ""
	m.inputs[3].SetValue("N/A")

	// Progress Note (simple textinput)
	m.progressInput = textinput.New()
	m.progressInput.Placeholder = "What did you work on today?"
	m.progressInput.CharLimit = 1000
	m.progressInput.Width = 60
	m.progressInput.Prompt = ""

	// Hours Input
	m.hoursInput = textinput.New()
	m.hoursInput.Placeholder = "0.0"
	m.hoursInput.CharLimit = 5
	m.hoursInput.Width = 60
	m.hoursInput.Prompt = ""
	m.hoursInput.Validate = func(s string) error {
		if s == "" {
			return nil
		}
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if val < 0 || val > 12 {
			return fmt.Errorf("must be between 0 and 12")
		}
		return nil
	}

	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

type issueTitleMsg struct {
	title string
	err   error
}

type submitResultMsg struct {
	success bool
	err     error
}

func fetchIssueTitle(issueURL string) tea.Cmd {
	return func() tea.Msg {
		title, err := getIssueTitle(issueURL)
		return issueTitleMsg{title: title, err: err}
	}
}

func submitForm(data formData, formURL string) tea.Cmd {
	return func() tea.Msg {
		err := submitToGoogleForm(data, formURL)
		if err != nil {
			return submitResultMsg{success: false, err: err}
		}
		return submitResultMsg{success: true, err: nil}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			if !m.submitted {
				return m, tea.Quit
			}

		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Can't navigate if submitted
			if m.submitted {
				if s == "enter" {
					return m, tea.Quit
				}
				return m, nil
			}

			// Submit on enter at submit button
			if s == "enter" && m.focusIndex == 6 {
				// Validate all fields
				if m.inputs[0].Value() == "" {
					m.err = fmt.Errorf("email is required")
					return m, nil
				}
				if m.inputs[2].Value() == "" {
					m.err = fmt.Errorf("issue link is required")
					return m, nil
				}
				if m.issueTitle == "" {
					m.err = fmt.Errorf("issue title is required")
					return m, nil
				}
				if m.progressInput.Value() == "" {
					m.err = fmt.Errorf("progress note is required")
					return m, nil
				}
				if m.hoursInput.Value() == "" {
					m.err = fmt.Errorf("hours spent is required")
					return m, nil
				}

				hours, err := strconv.ParseFloat(m.hoursInput.Value(), 64)
				if err != nil {
					m.err = fmt.Errorf("invalid hours value")
					return m, nil
				}

				data := formData{
					email:        m.inputs[0].Value(),
					date:         m.inputs[1].Value(),
					issueLink:    m.inputs[2].Value(),
					issueTitle:   m.issueTitle,
					progressNote: m.progressInput.Value(),
					projectName:  m.inputs[3].Value(),
					hoursSpent:   hours,
				}

				return m, submitForm(data, m.formURL)
			}

			// Fetch issue title when leaving issue link field
			if s == "enter" && m.focusIndex == 2 && m.inputs[2].Value() != "" && !m.fetching {
				m.fetching = true
				m.focusIndex++
				return m, fetchIssueTitle(m.inputs[2].Value())
			}

			// Cycle through inputs
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > 6 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = 6
			}

			cmds := make([]tea.Cmd, len(m.inputs)+1)
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}

			// Handle progress input focus (index 4)
			if m.focusIndex == 4 {
				cmds = append(cmds, m.progressInput.Focus())
			} else {
				m.progressInput.Blur()
			}

			// Handle hours input focus (index 5)
			if m.focusIndex == 5 {
				cmds = append(cmds, m.hoursInput.Focus())
			} else {
				m.hoursInput.Blur()
			}

			return m, tea.Batch(cmds...)
		}

	case issueTitleMsg:
		m.fetching = false
		if msg.err == nil && msg.title != "" {
			m.issueTitle = msg.title
		}
		return m, nil

	case submitResultMsg:
		if msg.success {
			m.submitted = true
			m.submitMsg = "✓ Report submitted successfully!"
		} else {
			m.err = msg.err
		}
		return m, nil
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)
	return m, cmd
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0)

	// Update all textinputs
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update progress input
	var cmd tea.Cmd
	m.progressInput, cmd = m.progressInput.Update(msg)
	cmds = append(cmds, cmd)

	// Update hours input
	m.hoursInput, cmd = m.hoursInput.Update(msg)
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
}

func (m model) View() string {
	if m.submitted {
		return fmt.Sprintf("\n  %s\n\n  Press enter to exit.\n\n", successStyle.Render(m.submitMsg))
	}

	var b strings.Builder

	// Title
	b.WriteString("\n  " + titleStyle.Render("daily report") + "\n\n")

	// Step indicator
	totalSteps := 7
	currentStep := m.focusIndex + 1
	if m.focusIndex == 6 {
		currentStep = 7
	}
	b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("Step %d of %d", currentStep, totalSteps)) + "\n\n")

	// Show completed fields summary
	if m.focusIndex > 0 {
		b.WriteString("  " + blurredStyle.Render("Completed:") + "\n")
		if m.focusIndex > 0 {
			b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Email: %s", m.inputs[0].Value())) + "\n")
		}
		if m.focusIndex > 1 {
			b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Date: %s", m.inputs[1].Value())) + "\n")
		}
		if m.focusIndex > 2 {
			b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Issue Link: %s", m.inputs[2].Value())) + "\n")
			if m.issueTitle != "" {
				b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Issue Title: %s", m.issueTitle)) + "\n")
			}
		}
		if m.focusIndex > 3 {
			b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Project: %s", m.inputs[3].Value())) + "\n")
		}
		if m.focusIndex > 4 {
			b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Progress: %s", m.progressInput.Value())) + "\n")
		}
		if m.focusIndex > 5 {
			b.WriteString("  " + blurredStyle.Render(fmt.Sprintf("✓ Hours: %s", m.hoursInput.Value())) + "\n")
		}
		b.WriteString("\n")
	}

	// Show current field
	switch m.focusIndex {
	case 0:
		b.WriteString("  " + focusedStyle.Render("> Email") + "\n")
		b.WriteString("  " + m.inputs[0].View() + "\n")
	case 1:
		b.WriteString("  " + focusedStyle.Render("> Date (DD/MM/YYYY)") + "\n")
		b.WriteString("  " + m.inputs[1].View() + "\n")
	case 2:
		b.WriteString("  " + focusedStyle.Render("> GitHub Issue Link") + "\n")
		b.WriteString("  " + m.inputs[2].View() + "\n")
	case 3:
		b.WriteString("  " + focusedStyle.Render("> Project Name") + "\n")
		b.WriteString("  " + m.inputs[3].View() + "\n")
	case 4:
		b.WriteString("  " + focusedStyle.Render("> Progress Note") + "\n")
		b.WriteString("  " + m.progressInput.View() + "\n")
	case 5:
		b.WriteString("  " + focusedStyle.Render("> Hours Spent") + "\n")
		b.WriteString("  " + m.hoursInput.View() + "\n")
	case 6:
		b.WriteString("  " + focusedButton + "\n")
	}

	// Error message
	if m.err != nil {
		b.WriteString(fmt.Sprintf("\n  %s\n", errorStyle.Render("✗ "+m.err.Error())))
	}

	// Status message
	if m.fetching {
		b.WriteString(fmt.Sprintf("\n  %s\n", blurredStyle.Render("⟳ Fetching issue title...")))
	}

	b.WriteString("\n  " + blurredStyle.Render("enter: next • ctrl+c: quit") + "\n\n")

	return b.String()
}

type formData struct {
	email        string
	date         string
	issueLink    string
	issueTitle   string
	progressNote string
	projectName  string
	hoursSpent   float64
}

func convertDateFormat(ddmmyyyy string) string {
	// Convert DD/MM/YYYY to MM/DD/YYYY
	parts := strings.Split(ddmmyyyy, "/")
	if len(parts) == 3 {
		return parts[1] + "/" + parts[0] + "/" + parts[2]
	}
	return ddmmyyyy
}

func submitToGoogleForm(data formData, formURL string) error {
	formValues := url.Values{}
	formValues.Add("entry.1426760171", convertDateFormat(data.date))
	formValues.Add("entry.183897134", data.issueLink)
	formValues.Add("entry.1622103326", data.issueTitle)
	formValues.Add("entry.2088302179", data.progressNote)
	formValues.Add("entry.1788320284", data.projectName)
	formValues.Add("entry.1879817041", fmt.Sprintf("%.1f", data.hoursSpent))
	formValues.Add("emailAddress", data.email)

	resp, err := http.PostForm(formURL, formValues)
	if err != nil {
		return fmt.Errorf("failed to submit form: %w", err)
	}
	defer resp.Body.Close()

	// Google Forms returns 200 or redirects on success
	if resp.StatusCode != 200 && resp.StatusCode != 302 && resp.StatusCode != 303 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func getGitEmail() string {
	cmd := exec.Command("git", "config", "user.email")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func getIssueTitle(issueURL string) (string, error) {
	// Extract owner/repo#number from GitHub URL
	parts := strings.Split(issueURL, "github.com/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub URL")
	}

	urlParts := strings.Split(parts[1], "/")
	if len(urlParts) < 4 {
		return "", fmt.Errorf("invalid GitHub URL format")
	}

	owner := urlParts[0]
	repo := urlParts[1]
	number := urlParts[3]

	issueRef := fmt.Sprintf("%s/%s#%s", owner, repo, number)

	cmd := exec.Command("gh", "issue", "view", issueRef, "--json", "title", "-q", ".title")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch issue title (is gh CLI installed?): %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(homeDir, ".config", "daily")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

func loadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func saveConfig(config *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func extractFormID(formURLInput string) (string, error) {
	pattern := `forms/d/e/([a-zA-Z0-9_-]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(formURLInput)

	if len(matches) < 2 {
		return "", fmt.Errorf("invalid Google Forms URL format")
	}

	return matches[1], nil
}

func buildFormResponseURL(formID string) string {
	return fmt.Sprintf("https://docs.google.com/forms/d/e/%s/formResponse", formID)
}

type setupModel struct {
	input textinput.Model
	err   error
}

func initialSetupModel() setupModel {
	ti := textinput.New()
	ti.Placeholder = "https://docs.google.com/forms/d/e/..."
	ti.Focus()
	ti.CharLimit = 300
	ti.Width = 70
	ti.Prompt = ""

	return setupModel{
		input: ti,
	}
}

func (m setupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			url := strings.TrimSpace(m.input.Value())
			if url == "" {
				m.err = fmt.Errorf("URL required")
				return m, nil
			}

			if !strings.Contains(url, "forms") {
				m.err = fmt.Errorf("invalid Google Forms URL")
				return m, nil
			}

			formID, err := extractFormID(url)
			if err != nil {
				m.err = err
				return m, nil
			}

			formResponseURL := buildFormResponseURL(formID)
			config := &Config{FormURL: formResponseURL}
			if err := saveConfig(config); err != nil {
				m.err = fmt.Errorf("could not save: %v", err)
				return m, nil
			}

			// Switch to main form
			return initialModel(formResponseURL), nil
		}
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m setupModel) View() string {
	var b strings.Builder

	b.WriteString("\n  " + titleStyle.Render("daily - setup") + "\n\n")
	b.WriteString("  " + focusedStyle.Render("▸ Google Forms URL") + "\n")
	b.WriteString("  " + m.input.View() + "\n\n")

	if m.err != nil {
		b.WriteString("  " + errorStyle.Render("✗ "+m.err.Error()) + "\n\n")
	}

	b.WriteString("  " + blurredStyle.Render("enter: continue • ctrl+c: quit") + "\n\n")

	return b.String()
}

func main() {
	config, err := loadConfig()
	if err != nil {
		config = &Config{}
	}

	var formResponseURL string

	if config.FormURL != "" {
		formID, err := extractFormID(config.FormURL)
		if err == nil {
			formResponseURL = buildFormResponseURL(formID)
		} else {
			// Invalid saved URL, run setup
			p := tea.NewProgram(initialSetupModel())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	} else {
		// No config, run setup
		p := tea.NewProgram(initialSetupModel())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	p := tea.NewProgram(initialModel(formResponseURL))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
