package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/robfig/cron/v3"
	
	"minion/internal/config"
	"minion/internal/engine"
	"minion/internal/store"
)

var (
	colorAccent  = lipgloss.Color("39")
	colorMuted   = lipgloss.Color("240")
	colorNormal  = lipgloss.Color("252")
	colorSuccess = lipgloss.Color("42")
	colorError   = lipgloss.Color("9")
	colorActive  = lipgloss.Color("13")

	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorNormal)
	borderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	
	headerStyle   = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(colorAccent).Bold(true).Padding(0, 1)
	normalStyle   = lipgloss.NewStyle().Foreground(colorNormal).Padding(0, 1)
	labelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("246")) // Calm, soft grey for keys
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	
	paneStyle     = lipgloss.NewStyle().Padding(0, 1)
)

func (m model) View() string {
	if m.width == 0 {
		return ""
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	mainH := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 2 // -2 for top and bottom margins
	if mainH < 0 {
		mainH = 0
	}

	isWide := m.width >= 80

	var content string

		if isWide {
			leftW := 56
			rightW := m.width - leftW - 1
		
		leftPane := paneStyle.Width(leftW - 2).Height(mainH).Render(m.renderList(leftW-4, mainH))
		
		var rightContent string
		switch m.state {
		case stateDashboard:
			rightContent = m.renderBuilder(rightW-4, mainH, false)
		case stateForm:
			rightContent = m.renderBuilder(rightW-4, mainH, true)
		case stateLogs:
			rightContent = m.renderLogs(rightW-4, mainH)
		case stateEnv:
			rightContent = m.renderEnvEditor(rightW-4, mainH)
		}
		rightPane := paneStyle.Width(rightW - 2).Height(mainH).Render(rightContent)

		var borderStr string
		if mainH > 0 {
			borderStr = strings.Repeat("│\n", mainH-1) + "│"
		}
		border := borderStyle.Render(borderStr)
		content = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, border, rightPane)
	} else {
		w := m.width - 2
		if w < 0 { w = 0 }
		innerW := w - 2
		if innerW < 0 { innerW = 0 }
		switch m.state {
		case stateDashboard:
			content = paneStyle.Width(w).Height(mainH).Render(m.renderList(innerW, mainH))
		case stateForm:
			content = paneStyle.Width(w).Height(mainH).Render(m.renderBuilder(innerW, mainH, true))
		case stateLogs:
			content = paneStyle.Width(w).Height(mainH).Render(m.renderLogs(innerW, mainH))
		case stateEnv:
			content = paneStyle.Width(w).Height(mainH).Render(m.renderEnvEditor(innerW, mainH))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, " ", header, content, footer, " ")
}

func (m model) DebugParts() (string, string, string, int) {
	header := m.renderHeader()
	footer := m.renderFooter()
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

func (m model) renderHeader() string {
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
	
	space := m.width - lipgloss.Width(title) - lipgloss.Width(status) - 4
	if space < 0 { space = 0 }
	
	header := fmt.Sprintf("  %s%s%s  ", title, strings.Repeat(" ", space), status)
	border := borderStyle.Render(strings.Repeat("─", m.width))
	
	return header + "\n" + border
}

	func (m model) renderFooter() string {
		border := borderStyle.Render(strings.Repeat("─", m.width))
		
		var helpView string
		if m.state == stateDashboard {
			if m.confirmDelete && !m.focusRight {
				filename := ""
				if len(m.minions) > 0 {
					filename = m.minions[m.cursor].Filename
				}
				helpView = lipgloss.NewStyle().Foreground(colorError).Bold(true).Render(fmt.Sprintf("[!] Delete minion file '%s'? Press 'y' to confirm, or 'esc' to cancel.", filename))
			} else if m.focusRight {
				m.help.ShortSeparator = " • "
				helpView = m.help.ShortHelpView(m.keys.RightFocusHelp())
			} else {
				m.help.ShortSeparator = " • "
				helpView = m.help.View(m.keys)
			}
		} else if m.state == stateForm {
			if m.confirmDelete {
				helpView = lipgloss.NewStyle().Foreground(colorError).Bold(true).Render("[!] Delete this item? Press 'y' to confirm, or 'esc' to cancel.")
			} else if m.editMode || m.addStepMode {
				helpView = mutedStyle.Render("enter Save • esc Cancel")
			} else {
				helpView = mutedStyle.Render("↑/k up • ↓/j down • shift+↑/↓ move • x delete • a add • enter edit • s save • esc cancel")
			}
		} else {
			m.help.ShortSeparator = " • "
			helpView = m.help.View(m.keys) // fallback
		}
	
		return border + "\n  " + helpView
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
		
		showStatus := w > 35
		
		schedW := 15
		nextW := 13 // Expanded to fit format "02 Jan 15:04"
	
		var headerLines []string
		if showStatus {
			nameW := w - 6 - schedW - nextW
			if nameW < 5 { nameW = 5 }
			
			nameHPad := nameW - lipgloss.Width("NAME")
			if nameHPad < 0 { nameHPad = 0 }
			
			schedHPad := schedW - lipgloss.Width("SCHEDULE")
			if schedHPad < 0 { schedHPad = 0 }
			
			headerLines = append(headerLines, fmt.Sprintf("    %s%s %s%s %s", 
				headerStyle.Render("NAME"), strings.Repeat(" ", nameHPad), 
				headerStyle.Render("SCHEDULE"), strings.Repeat(" ", schedHPad),
				headerStyle.Render("NEXT RUN")))
		} else {
			headerLines = append(headerLines, "  "+headerStyle.Render("NAME"))
		}
		headerLines = append(headerLines, "")
	
		var minionRows []string
		
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		now := time.Now()
		
		for i, mc := range m.minions {
			cursor := " "
			nameStyle := lipgloss.NewStyle().Foreground(colorNormal)
			
			if i == m.cursor {
				if m.focusRight {
					nameStyle = lipgloss.NewStyle().Foreground(colorNormal).Background(colorMuted).Bold(true)
					cursor = "▌"
				} else {
					// Minion Theme: Yellow text on Jeans Blue border indicator
					nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
					cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("68")).Render("▌")
				}
			}
	
			dbStore, _ := store.InitStore(config.DBPath)
			isStarted := false
			if dbStore != nil {
				isStarted = dbStore.GetMinionStatus(mc.Filename)
				dbStore.Close()
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
					// Need last run to calc next accurately, but now works roughly
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
			} else if isActive {
				// Use green play button for Running
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

			nameW := w - 4
			if showStatus {
				nameW = w - 6 - schedW - nextW
			}
			if nameW < 5 { nameW = 5 }
	
			name := mc.Name
			if lipgloss.Width(name) > nameW {
				name = name[:nameW-3] + "..."
			}
			
				namePad := nameW - lipgloss.Width(name)
				if namePad < 0 { namePad = 0 }
		
				var row string
				if showStatus {
					schedPad := schedW - lipgloss.Width(schedStatus)
					if schedPad < 0 { schedPad = 0 }
					
					nextPad := nextW - lipgloss.Width(nextStatus)
					if nextPad < 0 { nextPad = 0 }
					
					row = fmt.Sprintf(" %s %s%s%s %s%s %s%s", 
						dot, cursor, nameStyle.Render(name), strings.Repeat(" ", namePad), 
						schedStatus, strings.Repeat(" ", schedPad), 
						nextStatus, strings.Repeat(" ", nextPad))
				} else {
					row = fmt.Sprintf(" %s %s%s", dot, cursor, nameStyle.Render(name))
				}
			
			minionRows = append(minionRows, row)
		}
	
		start := m.listOffset
		end := start + (h - 2)
		if end > len(minionRows) { end = len(minionRows) }
		if start > end { start = end }
	
		visibleRows := append(headerLines, minionRows[start:end]...)
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
					rawMinion = selected // Fallback
				}
				d = initBuilderData(rawMinion)
			} else {
				return ""
			}
		}

	if d == nil {
		return ""
	}

	// For dashboard (read-only) view, we need to generate rows dynamically based on hovered minion
	var renderRows []builderRow
	if isEditMode {
		renderRows = m.builderRows
	} else {
		renderRows = generateBuilderRows(d)
	}

	var out strings.Builder

	title := "Configuration"
	if isEditMode {
		title = "Builder: " + d.Name
		if d.IsNew {
			title = "Builder: New Minion"
		}
	} else {
		title = d.Name
	}
	out.WriteString(titleStyle.Render(title) + "\n\n")

	if !isEditMode && m.state == stateDashboard {
		var actions string
			if m.focusRight {
				actions = mutedStyle.Render("Press ") + lipgloss.NewStyle().Foreground(colorAccent).Render("space") + mutedStyle.Render(" up/down • ") + 
						  lipgloss.NewStyle().Foreground(colorAccent).Render("r") + mutedStyle.Render(" run • ") + 
						  lipgloss.NewStyle().Foreground(colorAccent).Render("s") + mutedStyle.Render(" stop • ") + 
						  lipgloss.NewStyle().Foreground(colorAccent).Render("l") + mutedStyle.Render(" logs")
			} else {
			actions = mutedStyle.Render("Press Enter to view details & actions")
		}
		out.WriteString(actions + "\n\n")
	}

	// If we are actively editing the study task multiline
	if isEditMode && m.editMode && (renderRows[m.builderCursor].Field == "StudyTask" || renderRows[m.builderCursor].Field == "DeliverPayload" || renderRows[m.builderCursor].Field == "ReportPayload") {
		out.WriteString(headerStyle.Render("Edit: " + renderRows[m.builderCursor].Label) + "\n")
		out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
		
		m.textArea.SetWidth(w)
		m.textArea.SetHeight(h - 8)
		out.WriteString(m.textArea.View() + "\n")
		
		out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
		out.WriteString(mutedStyle.Render("esc Save & Return"))
		return out.String()
	}

	renderRow := func(i int, row builderRow) {
		if row.Type == rowSpacer {
			if row.Field == "EditOnlySpacer" && !isEditMode {
				return // Only show this spacer when in Edit Mode
			}
			out.WriteString("\n")
			return
		}

		cursor := "  "
		if isEditMode && m.builderCursor == i && !m.editMode && !m.addStepMode {
			cursor = "> "
		}

		if row.Type == rowAddStep {
			if isEditMode && m.addStepMode {
				cursor = "  "
				out.WriteString(fmt.Sprintf("\n%s%s\n", cursor, m.textInput.View()))
				
					sugs, _ := getSuggestions(true, "", "", m.textInput.Value())
				if len(sugs) > 0 {
						for j, s := range sugs {
							if j == 0 {
								out.WriteString(fmt.Sprintf("%s  %s\n", cursor, lipgloss.NewStyle().Foreground(colorAccent).Render(s)))
							} else {
							out.WriteString(fmt.Sprintf("%s  %s\n", cursor, mutedStyle.Render(s)))
						}
					}
				}
			} else if isEditMode {
				btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
				if m.builderCursor == i && !m.addStepMode && !m.editMode {
					btnStyle = selectedStyle
				}
				out.WriteString(fmt.Sprintf("\n%s%s\n", cursor, btnStyle.Render(row.Label)))
			}
			return
		}

		if row.Type == rowStepHeader {
			out.WriteString(fmt.Sprintf("\n%s%s\n", cursor, headerStyle.Render(row.Label)))
			return
		}

		indent := "  "
		if row.Type == rowStepField || row.Type == rowAddSubItem || row.Type == rowRemoveSubItem {
			indent = "    " // 4 spaces to align under the `- header`
		}
		
		cursor = indent + cursor

		if row.Type == rowAddSubItem || row.Type == rowRemoveSubItem {
			if isEditMode {
				btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Nice dark grey
				if m.builderCursor == i && !m.editMode && !m.addStepMode {
					btnStyle = selectedStyle
					
					// Make remove button red when selected
					if row.Type == rowRemoveSubItem {
						btnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("232")).Background(lipgloss.Color("1")).Bold(true).Padding(0, 1)
					}
				}
				
				// Calculate the padding to align perfectly with the values (15 spaces)
				padW := 15
				out.WriteString(fmt.Sprintf("%s%s%s\n", cursor, strings.Repeat(" ", padW), btnStyle.Render(row.Label)))
			}
			return
		}

		valW := w - lipgloss.Width(indent) - 15 - 4
		if valW < 5 { valW = 5 }

		var displayLines []string
		value := row.Value

		if isEditMode && m.editMode && m.builderCursor == i {
			cursor = indent + "  "
			m.textInput.Width = valW
			displayLines = []string{m.textInput.View()}
			
			contextType := ""
			if row.Field == "DeliverTarget" || row.Field == "ReportTarget" {
				if dStep, ok := m.builderData.Steps[row.StepIndex].(*DeliverStep); ok {
					contextType = dStep.Targets[row.TargetIndex].Type
				} else if rStep, ok := m.builderData.Steps[row.StepIndex].(*ReportStep); ok {
					contextType = rStep.Targets[row.TargetIndex].Type
				}
			}
			
			sugs, _ := getSuggestions(false, row.Field, contextType, m.textInput.Value())
			if len(sugs) > 0 {
					for j, s := range sugs {
						if j == 0 {
							displayLines = append(displayLines, lipgloss.NewStyle().Foreground(colorAccent).Render(s))
						} else {
						displayLines = append(displayLines, mutedStyle.Render(s))
					}
				}
			}
		} else if !isEditMode && row.Type != rowEnabled && (value == "" || value == "false") {
			return // Hide empty fields and false booleans in read-only mode
		} else if value == "" && isEditMode && row.Type != rowEnabled {
			if row.Field == "DeliverUsername" || row.Field == "DeliverPassword" || row.Field == "DeliverMethod" || row.Field == "DeliverHeaders" || row.Field == "DeliverPayload" ||
			   row.Field == "ReportUsername" || row.Field == "ReportPassword" || row.Field == "ReportMethod" || row.Field == "ReportHeaders" || row.Field == "ReportPayload" {
				displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(optional)")}
			} else {
				displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
			}
		} else {
			if row.Field == "StudyTask" || row.Field == "DeliverPayload" || row.Field == "ReportPayload" {
				lines := wrapLines(strings.TrimSpace(value), valW)
				for _, l := range lines {
					displayLines = append(displayLines, lipgloss.NewStyle().Foreground(colorNormal).Italic(true).Padding(0, 1).Render(l))
				}
				if len(displayLines) == 0 || (len(displayLines) == 1 && displayLines[0] == "") {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				}
			} else if row.Field == "SearchQueries" || row.Field == "FilterKeep" || row.Field == "FilterDrop" {
				queries := strings.Split(value, ",")
				for _, q := range queries {
					q = strings.TrimSpace(q)
					if q == "" { continue }
					lines := wrapLines(q, valW-2)
					if len(lines) > 0 {
						bullet := lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("- ")
						displayLines = append(displayLines, bullet + normalStyle.Render(lines[0]))
						for j := 1; j < len(lines); j++ {
							displayLines = append(displayLines, strings.Repeat(" ", lipgloss.Width(bullet)) + normalStyle.Render(lines[j]))
						}
					}
				}
				if len(displayLines) == 0 {
					displayLines = []string{lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1).Render("(none)")}
				}
				} else if row.Type == rowEnabled || row.Field == "BrowseRender" {
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
		if padW < 1 { padW = 1 }

		if len(displayLines) > 0 {
			labelStyleToUse := labelStyle
			if isEditMode && m.builderCursor == i && !m.editMode && !m.addStepMode {
				labelStyleToUse = selectedStyle
			}
			renderedLabel := labelStyleToUse.Render(row.Label)
			
			out.WriteString(fmt.Sprintf("%s%s%s%s\n", cursor, renderedLabel, strings.Repeat(" ", padW), displayLines[0]))
			for i := 1; i < len(displayLines); i++ {
				out.WriteString(fmt.Sprintf("%s%s%s%s\n", 
					strings.Repeat(" ", lipgloss.Width(cursor)), 
					strings.Repeat(" ", lipgloss.Width(renderedLabel)), 
					strings.Repeat(" ", padW), 
					displayLines[i]))
			}
		} else {
			labelStyleToUse := labelStyle
			if isEditMode && m.builderCursor == i && !m.editMode && !m.addStepMode {
				labelStyleToUse = selectedStyle
			}
			out.WriteString(fmt.Sprintf("%s%s\n", cursor, labelStyleToUse.Render(row.Label)))
		}
	}

	for i, row := range renderRows {
		if row.Type == rowStepHeader && i > 2 {
			if !isEditMode && i == 2 {
				out.WriteString("\n" + headerStyle.Render("mission") + "\n")
				out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
			}
		}
		
		if isEditMode && i == 2 {
			out.WriteString("\n" + headerStyle.Render("mission") + "\n")
			out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
		}

		renderRow(i, row)
	}

	out.WriteString("\n")
	return out.String()
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

	var missionSteps []string
	stepKeys := []string{"schedule", "search", "browse", "filter", "scrape", "study", "deliver", "receive", "report"}
	for _, step := range mc.Mission {
		stepAdded := false
		for _, k := range stepKeys {
			if _, ok := step[k]; ok {
				missionSteps = append(missionSteps, strings.Title(k))
				stepAdded = true
				break
			}
		}
		if !stepAdded {
			for key := range step {
				if key != "limit" && key != "keep" && key != "drop" {
					missionSteps = append(missionSteps, strings.Title(key))
					break
				}
			}
		}
	}
	if len(missionSteps) > 0 {
		out.WriteString(mutedStyle.Render("Mission: " + strings.Join(missionSteps, " -> ")) + "\n")
	}

	out.WriteString(borderStyle.Render(strings.Repeat("─", w)) + "\n")
	
	m.logViewport.Width = w
	
	vpH := h - 5
	if vpH < 0 { vpH = 0 }
	m.logViewport.Height = vpH
	
	out.WriteString(m.logViewport.View())

	return out.String()
}
