package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/robfig/cron/v3"
	
	"minion/internal/character"
	"minion/internal/engine"
)

var (
	colorAccent  = lipgloss.Color("39")
	colorMuted   = lipgloss.Color("240")
	colorNormal  = lipgloss.Color("252")
	colorSuccess = lipgloss.Color("42")
	colorError   = lipgloss.Color("9")
	colorActive  = lipgloss.Color("13")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorNormal)
	dirtyStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorError)
	borderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	
	headerStyle   = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(colorAccent).Bold(true).Padding(0, 1)
	normalStyle   = lipgloss.NewStyle().Foreground(colorNormal).Padding(0, 1)
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	subLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	
	paneStyle     = lipgloss.NewStyle().Padding(0, 1)

	cardBase = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("237")).
			Padding(1, 2)

	cardSepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
)

var stepColors = map[string]lipgloss.Color{
	"schedule": lipgloss.Color("74"),
	"search":   lipgloss.Color("114"),
	"browse":   lipgloss.Color("215"),
	"filter":   lipgloss.Color("140"),
	"scrape":   lipgloss.Color("209"),
	"study":    lipgloss.Color("104"),
	"deliver":  lipgloss.Color("167"),
	"report":   lipgloss.Color("176"),
}

func getStepColor(stepType string) lipgloss.Color {
	if c, ok := stepColors[stepType]; ok {
		return c
	}
	return colorAccent
}

var subFields = map[string]bool{
	"FromMatch":        true,
	"FromRender":       true,
	"FromLimit":        true,
	"FromFile":         true,
	"DeliverMarkdown":  true,
	"DeliverUsername":  true,
	"DeliverPassword":  true,
	"DeliverMethod":    true,
	"DeliverHeaders":   true,
	"DeliverPayload":   true,
	"DeliverCapacity":  true,
	"ReportMarkdown":   true,
	"ReportUsername":   true,
	"ReportPassword":  true,
	"ReportMethod":     true,
	"ReportHeaders":    true,
	"ReportPayload":    true,
	"ReportCapacity":   true,
}

func contentHeight(avail int) (_, h int) {
	h = avail
	if h < 3 { h = 3 }
	return
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	contentW := m.width - 2
	if contentW < 0 { contentW = 0 }
	innerW := contentW - 2
	if innerW < 0 { innerW = 0 }

	const minWidthForPet = 100
	showPet := m.width >= minWidthForPet

	header := m.renderHeader(contentW)
	footer := m.renderFooter(contentW)

	_, contentH := contentHeight(m.mainH)

	var content string

	if showPet && m.state == stateDashboard && character.Enabled() {
		const petH = 21
		if contentH >= petH+4 {
			petContent := paneStyle.Width(contentW).Height(petH).PaddingTop(0).Render(m.renderCharacter(innerW, petH))
			listH := contentH - petH - 1
			if listH < 3 {
				listH = 3
			}
			listContent := paneStyle.Width(contentW).Height(listH).PaddingTop(1).Render(m.renderList(innerW, listH))
			content = petContent + "\n" + listContent
		} else {
			content = paneStyle.Width(contentW).Height(contentH).PaddingTop(1).Render(m.renderList(innerW, contentH))
		}
	} else {
		switch m.state {
		case stateDashboard:
			content = paneStyle.Width(contentW).Height(contentH).PaddingTop(1).Render(m.renderList(innerW, contentH))
		case stateDetail:
			content = paneStyle.Width(contentW).Height(contentH).PaddingTop(1).Align(lipgloss.Center).Render(m.builderViewport.View())
		case stateForm:
			content = paneStyle.Width(contentW).Height(contentH).PaddingTop(1).Align(lipgloss.Center).Render(m.renderBuilder(innerW, contentH, true))
		case stateLogs:
			content = paneStyle.Width(contentW).Height(contentH).PaddingTop(1).Align(lipgloss.Center).Render(m.renderLogs(innerW, contentH))
		case stateEnv:
			content = paneStyle.Width(contentW).Height(contentH).PaddingTop(1).Align(lipgloss.Center).Render(m.renderEnvEditor(innerW, contentH))
		}
	}

	var body string
	if m.state == stateDetail || m.state == stateForm {
		body = header + content + "\n" + footer
	} else {
		body = header + "\n" + content + "\n" + footer
	}
	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(body)
}

func (m model) DebugParts() (string, string, string, int) {
	header := m.renderHeader(m.width)
	footer := m.renderFooter(m.width)
	mainH := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if mainH < 0 { mainH = 0 }
	
		isWide := m.width >= 80
		var content string
		if isWide {
			leftW := (m.width * 60) / 100
			if leftW < 56 { leftW = 56 }
			if leftW > 70 { leftW = 70 }
			rightW := m.width - leftW - 1
		leftPane := paneStyle.Width(leftW - 2).Height(mainH).Render(m.renderList(leftW-4, mainH))
		rightContent := m.renderBuilder(rightW-4, mainH, false)
		rightPane := paneStyle.Width(rightW - 2).Height(mainH).Render(rightContent)
		var borderStr string
		if mainH > 0 { borderStr = strings.Repeat("│\n", mainH-1) + "│" }
		border := borderStyle.Render(borderStr)
		
		var out strings.Builder
		for i, l := range strings.Split(rightContent, "\n") {
			if lipgloss.Width(l) > rightW - 4 {
				out.WriteString(fmt.Sprintf("Line %d is too long! Width: %d, Max: %d. Text: %s\n", i, lipgloss.Width(l), rightW-4, l))
			}
		}
		out.WriteString(fmt.Sprintf("Used: %d, %d, %d", lipgloss.Height(leftPane), lipgloss.Height(rightPane), lipgloss.Height(border)))
		content = out.String()
	}
	return header, footer, content, mainH
}

func (m model) renderHeader(w int) string {
	var status string
	activeCount := len(m.activeJobs)

	if m.upCount > 0 {
		upWord := "minions are"
		if m.upCount == 1 {
			upWord = "minion is"
		}
		upPart := fmt.Sprintf("%d %s up", m.upCount, upWord)

		if activeCount > 0 {
			runWord := "are running"
			if activeCount == 1 {
				runWord = "is running"
			}
			runPart := fmt.Sprintf("%d %s", activeCount, runWord)
			status = lipgloss.NewStyle().Foreground(colorSuccess).Render(upPart + ", " + runPart)
		} else {
			status = lipgloss.NewStyle().Foreground(colorSuccess).Render(upPart)
		}
	} else {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("All minions are down")
	}

	title := titleStyle.Render("Minions")

	innerW := w - 2
	space := innerW - lipgloss.Width(title) - lipgloss.Width(status)
	if space < 0 { space = 0 }

	barStyle := lipgloss.NewStyle().Padding(0, 1).Width(w)
	headerRow := barStyle.Render(title + strings.Repeat(" ", space) + status)
	border := borderStyle.Render(strings.Repeat("─", w))
	result := border + "\n" + headerRow + "\n" + border
	return result
}

	func (m model) renderFooter(w int) string {
		border := borderStyle.Render(strings.Repeat("─", w))
		
		var helpView string
		if m.state == stateDashboard {
			if m.confirmDelete {
				filename := ""
				if len(m.minions) > 0 {
					filename = m.minions[m.cursor].Filename
				}
				helpView = lipgloss.NewStyle().Foreground(colorError).Bold(true).Render(fmt.Sprintf("[!] Delete minion file '%s'? Press 'y' to confirm, or 'esc' to cancel.", filename))
			} else {
				m.help.ShortSeparator = " • "
				helpView = m.help.View(m.keys)
			}
		} else if m.state == stateDetail {
			helpView = mutedStyle.Render("esc back • space toggle • r run • s stop • l logs • e edit")
		} else if m.state == stateForm {
			if m.confirmDelete {
				helpView = lipgloss.NewStyle().Foreground(colorError).Bold(true).Render("[!] Delete this item? Press 'y' to confirm, or 'esc' to cancel.")
			} else if m.editMode || m.addStepMode || m.addSubItemField != "" {
				helpView = mutedStyle.Render("enter Save • esc Cancel")
			} else {
				helpView = mutedStyle.Render("↑/k up • ↓/j down • shift+↑/↓ move • x delete • a add • enter edit • s save • esc back")
				if m.dirty {
					helpView += "    " + dirtyStyle.Render("[unsaved]")
				}
			}
		} else {
			m.help.ShortSeparator = " • "
			helpView = m.help.View(m.keys)
		}

		footerLine := lipgloss.NewStyle().Padding(0, 1).Width(w).Render(helpView)
	return border + "\n" + footerLine + "\n" + border
}

	func formatNextRun(d time.Duration) string {
	if d < 0 {
		return "now"
	}
	if d < time.Minute {
		return fmt.Sprintf("in %ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("in %dh", h)
	}
	return fmt.Sprintf("in %dh %dm", h, m)
}

	func (m model) renderList(w, h int) string {
		if len(m.minions) == 0 {
			return mutedStyle.Render("No minions found.\nPress 'n' to create one.")
		}

		nameW := 30
		schedW := 13
		nextW := 11

		var headerLines []string
		nameHPad := nameW - lipgloss.Width("NAME")
		if nameHPad < 0 { nameHPad = 0 }
		schedHPad := schedW - lipgloss.Width("SCHEDULE")
		if schedHPad < 0 { schedHPad = 0 }
		headerLines = append(headerLines, fmt.Sprintf("    %s%s %s%s   %s",
			headerStyle.Render("NAME"), strings.Repeat(" ", nameHPad),
			headerStyle.Render("SCHEDULE"), strings.Repeat(" ", schedHPad),
			headerStyle.Render("NEXT RUN")))
		headerLines = append(headerLines, "")

		var minionRows []string

		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		now := time.Now()

		for i, mc := range m.minions {
			cursor := " "
			nameStyle := lipgloss.NewStyle().Foreground(colorNormal)

			if i == m.cursor {
				nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
				cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("68")).Render("▌")
			}

			isStarted := false
			if m.db != nil {
				isStarted = m.db.GetMinionStatus(mc.Filename)
			}

			isActive := m.activeJobs[mc.Filename]
			isDraft := false
			if mc.Enabled != nil && !*mc.Enabled {
				isDraft = true
			}

			var dot string
			var schedStatus string
			var nextStatus string

			rawSched := engine.ExtractSchedule(mc)
			cronExpr, _ := engine.ParseToCron(rawSched)
			var nextTime time.Time
			if cronExpr != "" {
				parsed, err := parser.Parse(cronExpr)
				if err == nil {
					nextTime = parsed.Next(now)
				}
			}

			if isDraft {
				if i != m.cursor {
					nameStyle = lipgloss.NewStyle().Foreground(colorMuted)
				}
				dot = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○")
				schedStatus = mutedStyle.Render("Disabled")
				nextStatus = mutedStyle.Render("-")
			} else if m.stoppingMinions[mc.Filename] {
				dot = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("▶")
				schedStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("Stopping...")
				nextStatus = mutedStyle.Render("-")
			} else if isActive {
				dot = lipgloss.NewStyle().Foreground(colorSuccess).Render("▶")
				schedStatus = lipgloss.NewStyle().Foreground(colorSuccess).Render("Running")
				nextStatus = mutedStyle.Render("-")
			} else if !isStarted {
				dot = mutedStyle.Render("○")
				schedStatus = mutedStyle.Render("Down")
				nextStatus = mutedStyle.Render("-")
			} else {
				if !m.daemonRunning {
					dot = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Render("■")
				} else {
					dot = mutedStyle.Render("●")
				}

				if rawSched == "" {
					schedStatus = mutedStyle.Render("Manual")
					nextStatus = mutedStyle.Render("-")
				} else {
					dispSched := rawSched
					if lipgloss.Width(dispSched) > schedW {
						dispSched = dispSched[:schedW-3] + "..."
					}
					schedStatus = mutedStyle.Render(dispSched)

					if nextTime.IsZero() {
						nextStatus = mutedStyle.Render("-")
					} else {
						if !m.daemonRunning {
							nextStatus = mutedStyle.Render(nextTime.Format("02 Jan 15:04"))
						} else {
							nextStatus = lipgloss.NewStyle().Foreground(colorNormal).Render(nextTime.Format("02 Jan 15:04"))
						}
					}
				}
			}

			name := mc.Name
			if lipgloss.Width(name) > nameW {
				name = name[:nameW-3] + "..."
			}
			namePad := nameW - lipgloss.Width(name)
			if namePad < 0 { namePad = 0 }

			schedPad := schedW - lipgloss.Width(schedStatus)
			if schedPad < 0 { schedPad = 0 }
			nextPad := nextW - lipgloss.Width(nextStatus)
			if nextPad < 0 { nextPad = 0 }

			row := fmt.Sprintf(" %s %s%s%s %s%s   %s%s",
				dot, cursor, nameStyle.Render(name), strings.Repeat(" ", namePad),
				schedStatus, strings.Repeat(" ", schedPad),
				nextStatus, strings.Repeat(" ", nextPad))

			minionRows = append(minionRows, row)
		}

		start := m.listOffset
		end := start + (h - 2)
		if end > len(minionRows) { end = len(minionRows) }
		if start > end { start = end }

		visibleRows := append(headerLines, minionRows[start:end]...)

		tableW := 1 + 1 + 1 + 1 + nameW + 1 + schedW + 3 + nextW
		pad := 0
		if w > tableW {
			pad = (w - tableW) / 2
		}
		if pad > 0 {
			prefix := strings.Repeat(" ", pad)
			for i, line := range visibleRows {
				visibleRows[i] = prefix + line
			}
		}

		return strings.Join(visibleRows, "\n")
	}

func (m model) renderBuilder(w, h int, isEditMode bool) string {
	return m.builderViewport.View()
}

func wrapLines(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		runes := []rune(line)
		if len(runes) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(runes) > 0 {
			if len(runes) <= width {
				lines = append(lines, string(runes))
				break
			}
			lines = append(lines, string(runes[:width]))
			runes = runes[width:]
		}
	}
	return lines
}

func (m model) renderBuilderString(w, h int, isEditMode bool) string {
	var d *builderData
	if isEditMode {
		d = m.builderData
	} else {
		if len(m.minions) > 0 {
			selected := m.minions[m.cursor]
			rawMinion := loadRawMinionForEditing(selected.Filename)
			if rawMinion == nil {
				rawMinion = selected
			}
			d = initBuilderData(rawMinion)
		} else {
			return ""
		}
	}

	if d == nil {
		return ""
	}

	var renderRows []builderRow
	if isEditMode {
		renderRows = m.builderRows
	} else {
		renderRows = generateBuilderRows(d)
	}

	var out strings.Builder

	title := "Configuration"
	if isEditMode {
		if d.IsNew {
			title = "New Minion"
		} else {
			title = d.Name
		}
	} else {
		title = d.Name
	}
	out.WriteString(titleStyle.Render(title) + "\n\n")

	if !isEditMode && (m.state == stateDashboard || m.state == stateDetail) {
		var actions string
		if m.state == stateDetail {
			actions = mutedStyle.Render("Press ") + lipgloss.NewStyle().Foreground(colorAccent).Render("space") + mutedStyle.Render(" toggle • ") +
				lipgloss.NewStyle().Foreground(colorAccent).Render("r") + mutedStyle.Render(" run • ") +
				lipgloss.NewStyle().Foreground(colorAccent).Render("s") + mutedStyle.Render(" stop • ") +
				lipgloss.NewStyle().Foreground(colorAccent).Render("l") + mutedStyle.Render(" logs • ") +
				lipgloss.NewStyle().Foreground(colorAccent).Render("e") + mutedStyle.Render(" edit")
		} else {
			actions = mutedStyle.Render("Press Enter to view details & actions")
		}
		out.WriteString(actions + "\n\n")
	}

	if isEditMode && m.editMode && (renderRows[m.builderCursor].Field == "StudyTask" || renderRows[m.builderCursor].Field == "DeliverPayload" || renderRows[m.builderCursor].Field == "ReportPayload") {
		out.WriteString(headerStyle.Render("Edit: "+renderRows[m.builderCursor].Label) + "\n")
		out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
		m.textArea.SetWidth(w)
		m.textArea.SetHeight(h - 8)
		out.WriteString(m.textArea.View() + "\n")
		out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
		out.WriteString(mutedStyle.Render("esc Save & Return"))
		return out.String()
	}

	type stepGroupData struct {
		stepIndex int
		stepType  string
		rows      []builderRow
	}

	var infoRows []builderRow
	var stepGroups []stepGroupData

	for _, row := range renderRows {
		switch row.Type {
		case rowStepHeader:
			stepGroups = append(stepGroups, stepGroupData{
				stepIndex: row.StepIndex,
				stepType:  row.Label,
				rows:      []builderRow{row},
			})
		default:
			if row.StepIndex == -1 || row.StepIndex == -2 {
				infoRows = append(infoRows, row)
			} else if len(stepGroups) > 0 {
				stepGroups[len(stepGroups)-1].rows = append(stepGroups[len(stepGroups)-1].rows, row)
			}
		}
	}

	const maxCardW = 120
	cardW := w - 6
	if cardW > maxCardW {
		cardW = maxCardW
	}
	if cardW < 20 {
		cardW = 20
	}
	leftPad := (w - cardW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	leftPadStr := strings.Repeat(" ", leftPad)

	cursorStepIndex := -2
	if isEditMode && m.builderCursor >= 0 && m.builderCursor < len(m.builderRows) {
		cursorStepIndex = m.builderRows[m.builderCursor].StepIndex
	}

	isStepActive := func(si int) bool {
		if m.builderCursor >= 0 && m.builderCursor < len(m.builderRows) &&
			m.builderRows[m.builderCursor].Type == rowAddStep {
			return false
		}
		return isEditMode && cursorStepIndex == si && !m.editMode && !m.addStepMode
	}

	renderField := func(target *strings.Builder, row builderRow, rowIdx int, availW int, stepColor lipgloss.Color) {
		if row.Type == rowSpacer || row.Type == rowStepHeader {
			return
		}

		cursor := "  "
		if isEditMode && m.builderCursor == rowIdx && !m.editMode && !m.addStepMode {
			cursor = "> "
		}

		indent := "  "
		if row.Type == rowStepField || row.Type == rowAddSubItem || row.Type == rowRemoveSubItem || row.Type == rowDeleteStep {
			indent = "  "
		}
		fullCursor := indent + cursor

		if row.Type == rowAddSubItem || row.Type == rowRemoveSubItem || row.Type == rowDeleteStep {
			if isEditMode {
				showInputPrompt := m.editMode && m.addSubItemField != "" && m.builderCursor == rowIdx && row.Type == rowAddSubItem
				if showInputPrompt {
					inputW := availW - lipgloss.Width(indent) - 15 - 4
					if inputW < 5 {
						inputW = 5
					}
					m.textInput.Width = inputW
					target.WriteString(fmt.Sprintf("%s%s\n", fullCursor, m.textInput.View()))
					var allSugs []string
				if m.addSubItemField == "From" {
					allSugs = []string{"url", "search", "minion", "command", "file"}
				} else {
					allSugs = []string{"ntfy", "discord", "http_request", "minion", "file"}
				}
				input := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
				var sugs []string
				for _, s := range allSugs {
					if strings.HasPrefix(s, input) {
						sugs = append(sugs, s)
					}
				}
					if len(sugs) > 0 {
						for j, s := range sugs {
							if j == 0 {
								target.WriteString(fmt.Sprintf("    %s\n", lipgloss.NewStyle().Foreground(colorAccent).Render(s)))
							} else {
								target.WriteString(fmt.Sprintf("    %s\n", mutedStyle.Render(s)))
							}
						}
					}
				} else {
					btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
					if m.builderCursor == rowIdx && !m.editMode && !m.addStepMode {
						btnStyle = selectedStyle
						if row.Type != rowAddSubItem {
							btnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color("1")).Bold(true).Padding(0, 1)
						}
					}
					if row.Type == rowDeleteStep {
						btn := btnStyle.Render(row.Label)
						line := cursor + btn
						lineW := lipgloss.Width(line)
						availW := cardW - 6
						pad := (availW - lineW) / 2
						if pad < 0 {
							pad = 0
						}
						target.WriteString(strings.Repeat(" ", pad) + line)
					} else {
						padW := 15
						target.WriteString(fmt.Sprintf("%s%s%s\n", fullCursor, strings.Repeat(" ", padW), btnStyle.Render(row.Label)))
					}
				}
			}
			return
		}

		valW := availW - lipgloss.Width(indent) - 15 - 4
		if valW < 5 {
			valW = 5
		}

		var displayLines []string
		value := row.Value

		if isEditMode && m.editMode && m.builderCursor == rowIdx {
			cursor = indent + "  "
			m.textInput.Width = valW
			displayLines = []string{m.textInput.View()}

			contextType := ""
			if (row.Field == "TellURL" || row.Field == "ReportURL") && row.StepIndex >= 0 && row.StepIndex < len(m.builderData.Steps) && row.TargetIndex >= 0 {
				if tellStep, ok := m.builderData.Steps[row.StepIndex].(*TellStep); ok {
					if row.TargetIndex < len(tellStep.Targets) {
						for k := range tellStep.Targets[row.TargetIndex] {
							if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" || k == "file" {
								contextType = k
								break
							}
						}
					}
				}
				if contextType == "" {
					if reportStep, ok := m.builderData.Steps[row.StepIndex].(*ReportStep); ok {
						if row.TargetIndex < len(reportStep.Targets) {
							for k := range reportStep.Targets[row.TargetIndex] {
								if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" || k == "file" {
									contextType = k
									break
								}
							}
						}
					}
				}
			}

			sugs, _ := getSuggestions(false, row.Field, contextType, m.textInput.Value())
			if len(sugs) > 0 {
				highlightIdx := m.sugHighlight
				if highlightIdx < 0 {
					highlightIdx = 0
				}
				for j, s := range sugs {
					if j == highlightIdx {
						displayLines = append(displayLines, lipgloss.NewStyle().Foreground(colorAccent).Render("> "+s))
					} else {
						displayLines = append(displayLines, mutedStyle.Render("  "+s))
					}
				}
			}
		} else if !isEditMode && row.Type != rowEnabled && (value == "" || value == "false") {
			return
		} else if value == "" && isEditMode && row.Type != rowEnabled {
			if row.Field == "DeliverMarkdown" || row.Field == "DeliverUsername" || row.Field == "DeliverPassword" || row.Field == "DeliverMethod" || row.Field == "DeliverHeaders" || row.Field == "DeliverPayload" ||
				row.Field == "ReportMarkdown" || row.Field == "ReportUsername" || row.Field == "ReportPassword" || row.Field == "ReportMethod" || row.Field == "ReportHeaders" || row.Field == "ReportPayload" {
				displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(optional)")}
			} else {
				displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
			}
		} else {
			if row.Field == "StudyTask" || row.Field == "DeliverPayload" || row.Field == "ReportPayload" {
				lines := wrapLines(strings.TrimSpace(value), valW)
				for _, l := range lines {
					displayLines = append(displayLines, lipgloss.NewStyle().Foreground(colorNormal).Padding(0, 1).Render(l))
				}
				if len(displayLines) == 0 || (len(displayLines) == 1 && displayLines[0] == "") {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				}
			} else if row.Field == "SearchQueries" || row.Field == "FilterKeep" || row.Field == "FilterDrop" {
				queries := strings.Split(value, ",")
				for _, q := range queries {
					q = strings.TrimSpace(q)
					if q == "" {
						continue
					}
					lines := wrapLines(q, valW-2)
					if len(lines) > 0 {
						bullet := lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("- ")
						displayLines = append(displayLines, bullet+normalStyle.Render(lines[0]))
						for j := 1; j < len(lines); j++ {
							displayLines = append(displayLines, strings.Repeat(" ", lipgloss.Width(bullet))+normalStyle.Render(lines[j]))
						}
					}
				}
				if len(displayLines) == 0 {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				}
			} else if row.Type == rowEnabled || row.Field == "BrowseRender" || row.Field == "DeliverMarkdown" || row.Field == "ReportMarkdown" {
				if value == "true" {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorSuccess).Padding(0, 1).Render("true")}
				} else {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorError).Padding(0, 1).Render("false")}
				}
			} else if row.Field == "Schedule" {
				if value == "" {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				} else {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorAccent).Padding(0, 1).Render(value)}
				}
			} else if row.Field == "BrowseURL" || row.Field == "TargetURL" ||
				row.Field == "TellURL" || row.Field == "ReportURL" ||
				row.Field == "DeliverTarget" || row.Field == "ReportTarget" {
				displayVal := strings.TrimSpace(value)
				if displayVal == "" {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				} else {
					plainStyle := lipgloss.NewStyle().Foreground(colorNormal)
					dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

					hostPart := displayVal
					pathPart := ""
					if u, err := url.Parse(displayVal); err == nil && u.Host != "" {
						hostPart = u.Scheme + "://" + u.Host
						pathPart = u.RequestURI()
						if pathPart == "/" {
							pathPart = ""
						}
					}

					full := hostPart + pathPart
					lines := wrapLines(full, valW)
					for i, l := range lines {
						if i == 0 && pathPart != "" && len(l) > len(hostPart) {
							h := l[:len(hostPart)]
							p := l[len(hostPart):]
							displayLines = append(displayLines, plainStyle.Render(h)+dimStyle.Render(p))
						} else if i == 0 {
							displayLines = append(displayLines, plainStyle.Render(l))
						} else if pathPart != "" {
							displayLines = append(displayLines, dimStyle.Render(l))
						} else {
							displayLines = append(displayLines, plainStyle.Render(l))
						}
					}
				}
			} else {
				displayVal := strings.TrimSpace(value)
				if displayVal == "" {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				} else {
					lines := wrapLines(displayVal, valW)
					for _, l := range lines {
						displayLines = append(displayLines, normalStyle.Render(l))
					}
				}
			}
		}

		padW := 15 - lipgloss.Width(row.Label)
		if padW < 1 {
			padW = 1
		}

		if len(displayLines) > 0 {
			labelStyleToUse := labelStyle
			if stepColor != "" {
				labelStyleToUse = lipgloss.NewStyle().Foreground(stepColor)
			}
			if subFields[row.Field] {
				labelStyleToUse = subLabelStyle
			}
			if isEditMode && m.builderCursor == rowIdx && !m.editMode && !m.addStepMode {
				labelStyleToUse = selectedStyle
			}
			renderedLabel := labelStyleToUse.Render(row.Label)

			target.WriteString(fmt.Sprintf("%s%s%s%s\n", fullCursor, renderedLabel, strings.Repeat(" ", padW), displayLines[0]))
			for i := 1; i < len(displayLines); i++ {
				target.WriteString(fmt.Sprintf("%s%s%s%s\n",
					strings.Repeat(" ", lipgloss.Width(fullCursor)),
					strings.Repeat(" ", lipgloss.Width(renderedLabel)),
					strings.Repeat(" ", padW),
					displayLines[i]))
			}
		} else {
			labelStyleToUse := labelStyle
			if stepColor != "" {
				labelStyleToUse = lipgloss.NewStyle().Foreground(stepColor)
			}
			if subFields[row.Field] {
				labelStyleToUse = subLabelStyle
			}
			if isEditMode && m.builderCursor == rowIdx && !m.editMode && !m.addStepMode {
				labelStyleToUse = selectedStyle
			}
			target.WriteString(fmt.Sprintf("%s%s\n", fullCursor, labelStyleToUse.Render(row.Label)))
		}
	}

	globalIdx := 0

	for _, row := range infoRows {
		if row.Type == rowAddStep {
			globalIdx++
			continue
		}
		renderField(&out, row, globalIdx, cardW, "")
		globalIdx++
	}

	out.WriteString("\n" + headerStyle.Render("mission") + "\n")
	out.WriteString(borderStyle.Render(strings.Repeat("─", cardW)) + "\n")

	if isEditMode {
		for _, row := range infoRows {
			if row.Type == rowAddStep && row.StepIndex == -2 {
				cursorOnThis := m.builderCursor >= 0 && m.builderCursor < len(m.builderRows) &&
					m.builderRows[m.builderCursor].Type == rowAddStep &&
					m.builderRows[m.builderCursor].StepIndex == -2

				if m.addStepMode && cursorOnThis {
					out.WriteString(fmt.Sprintf("  %s\n", m.textInput.View()))
					sugs, _ := getSuggestions(true, "", "", m.textInput.Value())
					if len(sugs) > 0 {
						for j, s := range sugs {
							if j == 0 {
								out.WriteString(fmt.Sprintf("    %s\n", lipgloss.NewStyle().Foreground(colorAccent).Render(s)))
							} else {
								out.WriteString(fmt.Sprintf("    %s\n", mutedStyle.Render(s)))
							}
						}
					}
				} else {
					btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
					prefix := "  "
					if cursorOnThis {
						btnStyle = selectedStyle
						prefix = "  > "
					}
					out.WriteString(fmt.Sprintf("%s%s\n", prefix, btnStyle.Render(row.Label)))
				}
				out.WriteString("  " + cardSepStyle.Render("│") + "\n")
				break
			}
		}
	}

	for gi, group := range stepGroups {
		if gi > 0 {
			conn := cardSepStyle.Render("│")
			out.WriteString("\n  " + conn + "\n")
		}

		var cardBuf strings.Builder

		stepColor := getStepColor(group.stepType)
		headerValue := ""
		headerField := ""
		if len(group.rows) > 0 && group.rows[0].Type == rowStepHeader {
			headerValue = group.rows[0].Value
			headerField = group.rows[0].Field
		}

		cursorOnHeader := isEditMode && m.builderCursor >= 0 && m.builderCursor < len(m.builderRows) &&
			m.builderRows[m.builderCursor].Type == rowStepHeader &&
			m.builderRows[m.builderCursor].StepIndex == group.stepIndex

		if isEditMode && cursorOnHeader && m.editMode && headerField != "" && m.builderRows[m.builderCursor].StepIndex == group.stepIndex {
			if headerField == "DoTask" {
				cardBuf.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render(group.stepType) + "\n")
				m.textArea.SetWidth(cardW - 14)
				cardBuf.WriteString("  " + m.textArea.View() + "\n")
			} else {
				m.textInput.Width = cardW - 20
				cardBuf.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render(group.stepType) + "  " + m.textInput.View() + "\n")
				if headerField == "When" {
					sugs, _ := getSuggestions(false, "When", "", m.textInput.Value())
					if len(sugs) > 0 {
						for j, s := range sugs {
							if j == 0 {
								cardBuf.WriteString("    " + lipgloss.NewStyle().Foreground(colorAccent).Render(s) + "\n")
							} else {
								cardBuf.WriteString("    " + mutedStyle.Render(s) + "\n")
							}
						}
					}
				}
			}
		} else if cursorOnHeader && headerField != "" {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
			if isEditMode {
				style = selectedStyle
			}
			if headerValue != "" {
				cardBuf.WriteString(style.Render(group.stepType) + ": " + lipgloss.NewStyle().Foreground(colorNormal).Render(headerValue) + "\n")
			} else {
				cardBuf.WriteString(style.Render(group.stepType) + ": " + lipgloss.NewStyle().Foreground(colorMuted).Render("(none)") + "\n")
			}
		} else if headerField != "" {
			if headerValue != "" {
				cardBuf.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render(group.stepType) + ": " + lipgloss.NewStyle().Foreground(colorNormal).Render(headerValue) + "\n")
			} else {
				cardBuf.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render(group.stepType) + "\n")
			}
		} else {
			cardBuf.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render(group.stepType) + "\n")
		}
		globalIdx++

		for _, row := range group.rows {
			switch row.Type {
			case rowStepHeader:
				continue
		case rowSpacer:
			cardBuf.WriteString("\n")
			globalIdx++
			continue
			case rowAddStep:
				globalIdx++
				continue
			}
			if row.Type == rowDeleteStep && isEditMode {
				cardBuf.WriteString("\n")
				sep := borderStyle.Render(strings.Repeat("─", cardW-6))
				cardBuf.WriteString("  " + sep + "\n")
			}
			renderField(&cardBuf, row, globalIdx, cardW-6, stepColor)
			globalIdx++
		}

		cardStyle := cardBase
		if isEditMode {
			cardStyle = cardBase.Padding(1, 2, 0, 2)
		}
		if isStepActive(group.stepIndex) {
			cardStyle = cardStyle.BorderForeground(colorAccent)
		}

		rendered := cardStyle.Width(cardW).Render(cardBuf.String())
		out.WriteString(rendered + "\n")

		if isEditMode {
			conn := cardSepStyle.Render("│")
			out.WriteString("  " + conn + "\n")

			cursorOnThis := m.builderCursor >= 0 && m.builderCursor < len(m.builderRows) &&
				m.builderRows[m.builderCursor].Type == rowAddStep &&
				m.builderRows[m.builderCursor].StepIndex == group.stepIndex

			if m.addStepMode && cursorOnThis {
				out.WriteString(fmt.Sprintf("  %s\n", m.textInput.View()))
				sugs, _ := getSuggestions(true, "", "", m.textInput.Value())
				if len(sugs) > 0 {
					for j, s := range sugs {
						if j == 0 {
							out.WriteString(fmt.Sprintf("    %s\n", lipgloss.NewStyle().Foreground(colorAccent).Render(s)))
						} else {
							out.WriteString(fmt.Sprintf("    %s\n", mutedStyle.Render(s)))
						}
					}
				}
			} else {
				btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
				prefix := "  "
				if cursorOnThis {
					btnStyle = selectedStyle
					prefix = "  > "
				}
				out.WriteString(fmt.Sprintf("%s%s\n", prefix, btnStyle.Render("[ + Add Step ]")))
			}
		}
	}

	for _, row := range infoRows {
		if row.Type != rowAddStep || row.StepIndex == -2 || !isEditMode {
			continue
		}
		cursorOnThis := m.builderCursor >= 0 && m.builderCursor < len(m.builderRows) &&
			m.builderRows[m.builderCursor].Type == rowAddStep &&
			m.builderRows[m.builderCursor].StepIndex == row.StepIndex

		if m.addStepMode && cursorOnThis {
			out.WriteString(fmt.Sprintf("\n  %s\n", m.textInput.View()))
			sugs, _ := getSuggestions(true, "", "", m.textInput.Value())
			if len(sugs) > 0 {
				for j, s := range sugs {
					if j == 0 {
						out.WriteString(fmt.Sprintf("    %s\n", lipgloss.NewStyle().Foreground(colorAccent).Render(s)))
					} else {
						out.WriteString(fmt.Sprintf("    %s\n", mutedStyle.Render(s)))
					}
				}
			}
		} else {
			btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			prefix := "  "
			if cursorOnThis {
				btnStyle = selectedStyle
				prefix = "  > "
			}
			out.WriteString(fmt.Sprintf("\n%s%s\n", prefix, btnStyle.Render(row.Label)))
		}
	}

	result := out.String()
	if leftPad > 0 {
		lines := strings.Split(result, "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = leftPadStr + line
			}
		}
		result = strings.Join(lines, "\n")
	}
	return result
}

func (m model) renderEnvEditor(w, h int) string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("Secrets & Environment (.env)") + "\n")
	out.WriteString(mutedStyle.Render("These variables can be injected into any minion using ${VAR_NAME}") + "\n")
	out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
	
	m.textArea.SetWidth(w)
	m.textArea.SetHeight(h - 6)
	out.WriteString(m.textArea.View() + "\n")
	
	out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
	out.WriteString(mutedStyle.Render("esc Save & Return"))
	return out.String()
}

func (m model) renderLogs(w, h int) string {
	if len(m.minions) == 0 {
		return ""
	}
	mc := m.minions[m.cursor]

	var out strings.Builder

	status := ""
	if m.activeJobs[mc.Filename] {
		status = m.logSpinner.View() + " Running"
	} else {
		status = "Logs"
	}

	out.WriteString(fmt.Sprintf("%s %s\n", titleStyle.Render(mc.Name), mutedStyle.Render(status)))

	out.WriteString(mutedStyle.Render("Task: "))
	if mc.Do != "" {
		lines := strings.Split(mc.Do, "\n")
		if len(lines) > 0 {
			out.WriteString(mutedStyle.Render(lines[0]))
			if len(lines) > 1 {
				out.WriteString(mutedStyle.Render(" ..."))
			}
		}
	}
	out.WriteString("\n")

	out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
	
	m.logViewport.Width = w
	
	vpH := h - 5
	if vpH < 0 { vpH = 0 }
	m.logViewport.Height = vpH
	
	out.WriteString(m.logViewport.View())

	return out.String()
}
