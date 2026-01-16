package main

import (
	"encoding/json"
	"fmt"
	"io"
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
)

func newModel(formURL string, mappings FieldMapping) model {
	m := model{
		inputs:        make([]textinput.Model, len(fields)),
		formURL:       formURL,
		fieldMappings: mappings,
	}

	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.placeholder
		ti.CharLimit = f.charLimit
		ti.Width = 60
		ti.Prompt = ""
		if f.defaultVal != nil {
			ti.SetValue(f.defaultVal())
		}
		if i == 0 {
			ti.Focus()
		}
		m.inputs[i] = ti
	}

	// Add hours validation
	m.inputs[4].Validate = func(s string) error {
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

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "enter":
			if m.opened {
				return m, tea.Quit
			}

			// Submit
			if m.focus == openIdx {
				return m.handleOpen()
			}

			if m.focus == 1 && m.inputs[1].Value() != "" && !m.fetching {
				m.fetching = true
				m.focus++
				return m, m.fetchIssueTitle()
			}

			m.focus++
			if m.focus > openIdx {
				m.focus = 0
			}
			return m, m.updateFocus()

		case "tab", "down":
			m.focus++
			if m.focus > openIdx {
				m.focus = 0
			}
			return m, m.updateFocus()

		case "shift+tab", "up":
			m.focus--
			if m.focus < 0 {
				m.focus = openIdx
			}
			return m, m.updateFocus()
		}

	case issueTitleMsg:
		m.fetching = false
		if msg.errMsg != "" {
			m.err = fmt.Errorf(msg.errMsg)
		} else if msg.title != "" {
			m.issueTitle = msg.title
			m.err = nil
		} else {
			m.err = fmt.Errorf("issue title is empty")
		}
		return m, nil

	case openResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.opened = true
		}
		return m, nil
	}

	// Update current input
	if m.focus < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) updateFocus() tea.Cmd {
	for i := range m.inputs {
		if i == m.focus {
			return m.inputs[i].Focus()
		}
		m.inputs[i].Blur()
	}
	return nil
}

func (m model) handleOpen() (tea.Model, tea.Cmd) {
	if m.inputs[1].Value() == "" {
		m.err = fmt.Errorf("issue link is required")
		return m, nil
	}
	if m.issueTitle == "" {
		m.err = fmt.Errorf("issue title not fetched - run 'gh auth login' first")
		return m, nil
	}
	if m.inputs[3].Value() == "" {
		m.err = fmt.Errorf("progress note is required")
		return m, nil
	}
	if m.inputs[4].Value() == "" {
		m.err = fmt.Errorf("hours spent is required")
		return m, nil
	}

	hours, err := strconv.ParseFloat(m.inputs[4].Value(), 64)
	if err != nil {
		m.err = fmt.Errorf("invalid hours value")
		return m, nil
	}

	return m, func() tea.Msg {
		err := openPrefilledForm(
			m.inputs[0].Value(), // date
			m.inputs[1].Value(), // issueLink
			m.issueTitle,
			m.inputs[3].Value(), // progressNote
			m.inputs[2].Value(), // projectName
			hours,
			m.formURL,
			m.fieldMappings,
		)
		return openResultMsg{err: err}
	}
}

func (m model) fetchIssueTitle() tea.Cmd {
	return func() tea.Msg {
		title, err := getIssueTitle(m.inputs[1].Value())
		if err != nil {
			return issueTitleMsg{errMsg: err.Error()}
		}
		return issueTitleMsg{title: title}
	}
}

func (m model) View() string {
	if m.opened {
		return fmt.Sprintf("\n  %s\n\n  Press enter to exit.\n\n",
			successStyle.Render("✓ Pre-filled form opened in browser!"))
	}

	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render("daily report") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s\n\n", blurredStyle.Render(fmt.Sprintf("Step %d of %d", m.focus+1, openIdx+1))))

	// Completed fields
	if m.focus > 0 {
		b.WriteString("  " + blurredStyle.Render("Completed:") + "\n")
		for i := 0; i < m.focus && i < len(fields); i++ {
			val := m.inputs[i].Value()
			b.WriteString(fmt.Sprintf("  %s\n", blurredStyle.Render(fmt.Sprintf("✓ %s: %s", fields[i].name, val))))
			// Show issue title after issue link
			if i == 1 && m.issueTitle != "" {
				b.WriteString(fmt.Sprintf("  %s\n", blurredStyle.Render(fmt.Sprintf("✓ Issue Title: %s", m.issueTitle))))
			}
		}
		b.WriteString("\n")
	}

	// Current field
	if m.focus < len(fields) {
		b.WriteString(fmt.Sprintf("  %s\n", focusedStyle.Render("> "+fields[m.focus].name)))
		b.WriteString("  " + m.inputs[m.focus].View() + "\n")
	} else {
		b.WriteString("  " + focusedStyle.Render("[ Open Pre-filled Form ]") + "\n")
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("\n  %s\n", errorStyle.Render("✗ "+m.err.Error())))
	}
	if m.fetching {
		b.WriteString(fmt.Sprintf("\n  %s\n", blurredStyle.Render("⟳ Fetching issue title...")))
	}

	b.WriteString("\n  " + blurredStyle.Render("enter: next • ctrl+c: quit") + "\n\n")
	return b.String()
}

func openPrefilledForm(date, issueLink, issueTitle, progressNote, projectName string, hours float64, formURL string, mappings FieldMapping) error {
	viewFormURL := strings.Replace(formURL, "/formResponse", "/viewform", 1)

	v := url.Values{}
	v.Add("usp", "pp_url")
	v.Add("entry."+mappings.Date, convertDateFormat(date))
	v.Add("entry."+mappings.IssueLink, issueLink)
	v.Add("entry."+mappings.IssueTitle, issueTitle)
	v.Add("entry."+mappings.ProgressNote, progressNote)
	v.Add("entry."+mappings.ProjectName, projectName)
	v.Add("entry."+mappings.HoursSpent, fmt.Sprintf("%.1f", hours))

	return exec.Command("open", viewFormURL+"?"+v.Encode()).Run()
}

func convertDateFormat(ddmmyyyy string) string {
	parts := strings.Split(ddmmyyyy, "/")
	if len(parts) == 3 {
		return parts[2] + "-" + parts[1] + "-" + parts[0] // YYYY-MM-DD
	}
	return ddmmyyyy
}

func getIssueTitle(issueURL string) (string, error) {
	cmd := exec.Command("gh", "issue", "view", strings.TrimSpace(issueURL), "--json", "title", "-q", ".title")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(output)); msg != "" {
			return "", fmt.Errorf("gh: %s", msg)
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Setup model methods
func newSetupModel() setupModel {
	ti := textinput.New()
	ti.Placeholder = "https://docs.google.com/forms/d/e/..."
	ti.Focus()
	ti.CharLimit = 300
	ti.Width = 70
	ti.Prompt = ""
	return setupModel{input: ti}
}

func (m setupModel) Init() tea.Cmd { return textinput.Blink }

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.loading {
		if msg, ok := msg.(formFieldsMsg); ok {
			if msg.err != nil {
				m.err = msg.err
				m.loading = false
				return m, nil
			}
			mapping := mapFieldsByOrder(msg.fields)
			if !isFieldMappingComplete(mapping) {
				m.err = fmt.Errorf("form needs at least 6 fields")
				m.loading = false
				return m, nil
			}
			saveConfig(&Config{FormURL: m.formURL, FieldMappings: mapping})
			return newModel(m.formURL, mapping), nil
		}
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			inputURL := strings.TrimSpace(m.input.Value())
			if inputURL == "" || !strings.Contains(inputURL, "forms") {
				m.err = fmt.Errorf("invalid Google Forms URL")
				return m, nil
			}
			formID, err := extractFormID(inputURL)
			if err != nil {
				m.err = err
				return m, nil
			}
			m.formURL = fmt.Sprintf("https://docs.google.com/forms/d/e/%s/formResponse", formID)
			m.loading = true
			m.err = nil
			return m, fetchFormFieldsCmd(m.formURL)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m setupModel) View() string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render("daily - setup") + "\n\n")

	if m.loading {
		b.WriteString("  " + blurredStyle.Render("Fetching form fields...") + "\n\n")
	} else {
		b.WriteString("  " + focusedStyle.Render("> Google Forms URL") + "\n")
		b.WriteString("  " + m.input.View() + "\n\n")
		if m.err != nil {
			b.WriteString("  " + errorStyle.Render("✗ "+m.err.Error()) + "\n\n")
		}
		b.WriteString("  " + blurredStyle.Render("enter: continue • ctrl+c: quit") + "\n\n")
	}
	return b.String()
}

// Form field detection
func fetchFormFieldsCmd(formURL string) tea.Cmd {
	return func() tea.Msg {
		viewURL := strings.Replace(formURL, "/formResponse", "/viewform", 1)
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(viewURL)
		if err != nil {
			return formFieldsMsg{err: err}
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		fields, err := parseFormFields(string(body))
		return formFieldsMsg{fields: fields, err: err}
	}
}

func parseFormFields(html string) ([]FormField, error) {
	start := strings.Index(html, "var FB_PUBLIC_LOAD_DATA_ = ")
	if start == -1 {
		return nil, fmt.Errorf("FB_PUBLIC_LOAD_DATA_ not found")
	}
	start += len("var FB_PUBLIC_LOAD_DATA_ = ")
	end := strings.Index(html[start:], ";</script>")
	if end == -1 {
		return nil, fmt.Errorf("could not parse form data")
	}

	var data []interface{}
	if err := json.Unmarshal([]byte(html[start:start+end]), &data); err != nil {
		return nil, err
	}

	formData, _ := data[1].([]interface{})
	questions, _ := formData[1].([]interface{})

	var fields []FormField
	for _, q := range questions {
		question, _ := q.([]interface{})
		if len(question) < 5 {
			continue
		}
		label, _ := question[1].(string)
		entryWrapper, _ := question[4].([]interface{})
		if len(entryWrapper) == 0 {
			continue
		}
		entryInner, _ := entryWrapper[0].([]interface{})
		if len(entryInner) == 0 {
			continue
		}
		if id, ok := entryInner[0].(float64); ok {
			fields = append(fields, FormField{ID: fmt.Sprintf("%.0f", id), Label: label})
		}
	}
	return fields, nil
}

func mapFieldsByOrder(fields []FormField) FieldMapping {
	if len(fields) < 6 {
		return FieldMapping{}
	}
	return FieldMapping{
		Date:         fields[0].ID,
		IssueLink:    fields[1].ID,
		IssueTitle:   fields[2].ID,
		ProgressNote: fields[3].ID,
		ProjectName:  fields[4].ID,
		HoursSpent:   fields[5].ID,
	}
}

func isFieldMappingComplete(m FieldMapping) bool {
	return m.Date != "" && m.IssueLink != "" && m.IssueTitle != "" &&
		m.ProgressNote != "" && m.ProjectName != "" && m.HoursSpent != ""
}

// Config management
func loadConfig() (*Config, error) {
	path, _ := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{}, nil
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, _ := configPath()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func configPath() (string, error) {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "daily")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json"), nil
}

func extractFormID(url string) (string, error) {
	re := regexp.MustCompile(`forms/d/e/([a-zA-Z0-9_-]+)`)
	if m := re.FindStringSubmatch(url); len(m) >= 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("invalid Google Forms URL")
}

func main() {
	cfg, _ := loadConfig()

	if cfg.FormURL != "" && isFieldMappingComplete(cfg.FieldMappings) {
		if id, err := extractFormID(cfg.FormURL); err == nil {
			formURL := fmt.Sprintf("https://docs.google.com/forms/d/e/%s/formResponse", id)
			tea.NewProgram(newModel(formURL, cfg.FieldMappings)).Run()
			return
		}
	}

	tea.NewProgram(newSetupModel()).Run()
}
