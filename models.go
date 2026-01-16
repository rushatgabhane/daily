package main

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	titleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
)

// Field definitions for the wizard
type fieldDef struct {
	name        string
	placeholder string
	charLimit   int
	defaultVal  func() string
}

var fields = []fieldDef{
	{"Date (DD/MM/YYYY)", "DD/MM/YYYY", 10, func() string { return time.Now().Format("02/01/2006") }},
	{"GitHub Issue Link", "https://github.com/...", 200, nil},
	{"Project Name", "N/A", 100, func() string { return "N/A" }},
	{"Progress Note", "What did you work on today?", 1000, nil},
	{"Hours Spent", "0.0", 5, nil},
}

const openIdx = 5 // Index of open button

// Config types
type FieldMapping struct {
	Date         string `json:"date,omitempty"`
	IssueLink    string `json:"issue_link,omitempty"`
	IssueTitle   string `json:"issue_title,omitempty"`
	ProgressNote string `json:"progress_note,omitempty"`
	ProjectName  string `json:"project_name,omitempty"`
	HoursSpent   string `json:"hours_spent,omitempty"`
}

type Config struct {
	FormURL       string       `json:"form_url"`
	FieldMappings FieldMapping `json:"field_mappings,omitempty"`
}

// Main form model
type model struct {
	inputs        []textinput.Model
	focus         int
	issueTitle    string
	err           error
	opened        bool
	fetching      bool
	formURL       string
	fieldMappings FieldMapping
}

// Messages
type issueTitleMsg struct{ title, errMsg string }
type openResultMsg struct{ err error }

// Setup model for first-time configuration
type setupModel struct {
	input   textinput.Model
	err     error
	loading bool
	formURL string
}

// Form field detection
type FormField struct{ ID, Label string }
type formFieldsMsg struct {
	fields []FormField
	err    error
}
