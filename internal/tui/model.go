package tui

import (
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"minion/internal/config"
	"minion/internal/store"
)

type sessionState int

const (
	stateDashboard sessionState = iota
	stateDetail
	stateLogs
	stateForm
	stateEnv
)

type builderRowType int

const (
	rowName builderRowType = iota
	rowEnabled
	rowStepHeader
	rowStepField
	rowAddStep
	rowAddSubItem
	rowRemoveSubItem
	rowDeleteStep
	rowSpacer
)

type builderRow struct {
	Type        builderRowType
	StepIndex   int
	TargetIndex int // Used for Browse targets
	Field       string
	Label       string
	Value       string
}

type model struct {
	state sessionState

	minions       []*config.MinionConfig
	activeJobs    map[string]bool
	upCount       int
	daemonRunning bool
	cursor        int

	width  int
	height int

	mainH  int

	logViewport     viewport.Model
	builderViewport viewport.Model
	logSpinner      spinner.Model
	tailing         bool
	logContent      string

	help help.Model
	keys keyMap

	builderData   *builderData
	builderRows   []builderRow
	builderCursor int
	builderOffset int
	listOffset    int
	editMode      bool
	addStepMode   bool
	confirmDelete bool
	dirty         bool
	textInput     textinput.Model
	textArea      textarea.Model

	db *store.Store
}

type PublicModel interface {
	tea.Model
	UpdatePublic(msg tea.Msg) (tea.Model, tea.Cmd)
	DebugParts() (string, string, string, int)
}

func NewModelExported() PublicModel {
	return newModel()
}

func ForceEmptyBuilder(m interface{}) {
}

func newModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	ti := textinput.New()
	ti.Prompt = "> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = "┃ "
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	bv := viewport.New(0, 0)
	bv.KeyMap.Up.SetEnabled(false)
	bv.KeyMap.Down.SetEnabled(false)
	// We want mouse wheel and page up/page down to work
	bv.KeyMap.HalfPageUp.SetEnabled(true)
	bv.KeyMap.HalfPageDown.SetEnabled(true)
	bv.KeyMap.PageUp.SetEnabled(true)
	bv.KeyMap.PageDown.SetEnabled(true)

	db, err := store.InitStore(config.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to open store: %v\n", err)
	}

	m := model{
		state:           stateDashboard,
		logSpinner:      s,
		help:            help.New(),
		keys:            keys,
		textInput:       ti,
		textArea:        ta,
		builderViewport: bv,
		db:              db,
	}

	m.loadState()
	return m
}

func (m model) UpdatePublic(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.Update(msg)
}

func (m *model) loadState() {
	allMinions, _ := config.LoadAllMinions()
	m.minions = allMinions

	sort.SliceStable(m.minions, func(i, j int) bool {
		iDisabled := m.minions[i].Enabled != nil && !*m.minions[i].Enabled
		jDisabled := m.minions[j].Enabled != nil && !*m.minions[j].Enabled
		return !iDisabled && jDisabled
	})

	if _, err := os.Stat(config.PIDPath); err == nil {
		m.daemonRunning = true
	} else {
		m.daemonRunning = false
	}

	if m.db != nil {
		m.activeJobs, _ = m.db.GetActiveJobs()
		m.upCount = 0
		activeMinions, _ := m.db.GetActiveMinions()
		for _, mc := range m.minions {
			if mc.Enabled != nil && !*mc.Enabled {
				continue
			}
			if activeMinions[mc.Filename] {
				m.upCount++
			}
		}
	} else {
		m.activeJobs = make(map[string]bool)
		m.upCount = 0
	}

	if len(m.minions) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.minions) {
		m.cursor = len(m.minions) - 1
	} else if m.cursor < 0 {
		m.cursor = 0
	}
}

func generateBuilderRows(data *builderData) []builderRow {
	var rows []builderRow
	if data == nil {
		return rows
	}
	
	rows = append(rows, builderRow{Type: rowName, StepIndex: -1, TargetIndex: -1, Field: "Name", Label: "name", Value: data.Name})
	rows = append(rows, builderRow{Type: rowEnabled, StepIndex: -1, TargetIndex: -1, Field: "Enabled", Label: "enabled", Value: fmt.Sprintf("%t", data.Enabled)})
	
	for i, step := range data.Steps {
		rows = append(rows, builderRow{Type: rowStepHeader, StepIndex: i, TargetIndex: -1, Label: string(step.Type())})
		rows = append(rows, step.GetRows(i)...)
		rows = append(rows, builderRow{Type: rowDeleteStep, StepIndex: i, TargetIndex: -1, Label: "[ - Delete Step ]"})
		rows = append(rows, builderRow{Type: rowAddStep, StepIndex: i, TargetIndex: -1, Label: "[ + Add Step ]"})
	}
	if len(data.Steps) == 0 {
		rows = append(rows, builderRow{Type: rowAddStep, StepIndex: -1, TargetIndex: -1, Label: "[ + Add Step ]"})
	}
	return rows
}

func (m *model) refreshBuilderRows() {
	if m.builderData == nil {
		return
	}
	
	m.builderRows = generateBuilderRows(m.builderData)
	if m.builderCursor >= len(m.builderRows) {
		m.builderCursor = len(m.builderRows) - 1
	}
	if m.builderCursor < 0 {
		m.builderCursor = 0
	}
	m.dirty = true
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.logSpinner.Tick, textinput.Blink, tickActiveJobs())
}
