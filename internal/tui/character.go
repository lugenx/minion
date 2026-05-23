package tui

import (
	"fmt"
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss"

	"minion/internal/character"
	"minion/internal/config"
)

var zzzSeq = []string{"",   "z",  "zz", "zzZ", "zzZ.", "zzZ..", "zzZ..."}

func (m model) renderCharacter(w, h int) string {
	if !character.Enabled() {
		return lipgloss.NewStyle().Width(w).Height(h).Render("")
	}

	fn := m.focusFilename
	if fn == "" && len(m.minions) > 0 && m.cursor < len(m.minions) {
		fn = m.minions[m.cursor].Filename
	}
	if fn == "" {
		return lipgloss.NewStyle().Width(w).Height(h).Render("")
	}

	var mc *config.MinionConfig
	for _, c := range m.minions {
		if c.Filename == fn {
			mc = c
			break
		}
	}
	if mc == nil {
		return lipgloss.NewStyle().Width(w).Height(h).Render("")
	}

	pd := m.characterStates[fn]
	if pd.HairStyle == "" {
		return lipgloss.NewStyle().Width(w).Height(h).Render("")
	}

	isStarted := false
	if m.db != nil {
		isStarted = m.db.GetMinionStatus(fn)
	}

	disabled := mc.Enabled != nil && !*mc.Enabled

	stage := character.GetStage(pd.CreatedAt)
	mood := character.GetMood(disabled, isStarted)
	if mood == character.Awake && m.isBlinking {
		mood = character.Blinking
	}
	lines := character.Art(pd.HairStyle, stage, mood)
	if len(lines) == 0 {
		return lipgloss.NewStyle().Width(w).Height(h).Render("")
	}

	for i := range lines {
		lines[i] = character.ColorLine(stage, i, lines[i])
	}

	grey := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	const artBoxH = 10
	for len(lines) < artBoxH {
		if mood == character.Sleeping && len(lines) == artBoxH-1 {
			lines = append([]string{strings.Repeat(" ", 22) + grey.Render(fmt.Sprintf("%-6s", zzzSeq[m.charTick%len(zzzSeq)]))}, lines...)
		} else {
			lines = append([]string{""}, lines...)
		}
	}

	ageStr := ""
	if !pd.CreatedAt.IsZero() {
		ageStr = character.FormatAge(pd.CreatedAt)
	}

	labelSty := lipgloss.NewStyle().Foreground(colorMuted)
	valueSty := lipgloss.NewStyle().Foreground(colorNormal)

	nameLine := labelSty.Render("Name:") + " " + valueSty.Render(mc.Name)
	ageLine := labelSty.Render("Age:") + " " + valueSty.Render(ageStr)
	xpLine := labelSty.Render("Missions:") + " " + valueSty.Render(fmt.Sprintf("%d", pd.TotalRuns))

	bossStr := "None"
	if bossNames := m.bosses[fn]; len(bossNames) > 0 {
		bossStr = strings.Join(bossNames, ", ")
	}
	bossLine := labelSty.Render("Boss:") + " " + valueSty.Render(bossStr)

	workerStr := "None"
	if workerNames := m.workers[fn]; len(workerNames) > 0 {
		workerStr = strings.Join(workerNames, ", ")
	}
	workerLine := labelSty.Render("Worker:") + " " + valueSty.Render(workerStr)

	const minTextW = 20
	textW := lipgloss.Width(nameLine)
	if w := lipgloss.Width(ageLine); w > textW {
		textW = w
	}
	if w := lipgloss.Width(xpLine); w > textW {
		textW = w
	}
	if w := lipgloss.Width(bossLine); w > textW {
		textW = w
	}
	if w := lipgloss.Width(workerLine); w > textW {
		textW = w
	}
	if textW < minTextW {
		textW = minTextW
	}
	nameLine += strings.Repeat(" ", textW-lipgloss.Width(nameLine))
	ageLine += strings.Repeat(" ", textW-lipgloss.Width(ageLine))
	xpLine += strings.Repeat(" ", textW-lipgloss.Width(xpLine))
	bossLine += strings.Repeat(" ", textW-lipgloss.Width(bossLine))
	workerLine += strings.Repeat(" ", textW-lipgloss.Width(workerLine))

	const textBoxH = 6
	textLines := []string{nameLine, ageLine, xpLine, bossLine, workerLine}
	textPad := (textBoxH - len(textLines)) / 2
	for i := 0; i < textPad; i++ {
		textLines = append([]string{""}, textLines...)
	}
	for len(textLines) < textBoxH {
		textLines = append(textLines, "")
	}

	var allLines []string
	allLines = append(allLines, "")
	allLines = append(allLines, lines...)
	allLines = append(allLines, "")
	allLines = append(allLines, textLines...)
	allLines = append(allLines, "")

	tableW := 62
	combined := strings.Join(allLines, "\n")

	pad := (w - (tableW + 2)) / 2
	if pad < 0 {
		pad = 0
	}

	frameStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorMuted).
		Padding(0, 1).
		Width(tableW).
		Align(lipgloss.Center)

	framedArt := frameStyle.Render(combined)
	boxH := lipgloss.Height(framedArt)

	topPad := (h - boxH) / 3
	if topPad < 0 {
		topPad = 0
	}
	bottomPad := h - topPad - boxH
	if bottomPad < 0 {
		bottomPad = 0
	}

	var result strings.Builder
	for i := 0; i < topPad; i++ {
		result.WriteString("\n")
	}
	for _, line := range strings.Split(framedArt, "\n") {
		result.WriteString(strings.Repeat(" ", pad))
		result.WriteString(line)
		result.WriteString("\n")
	}
	for i := 0; i < bottomPad; i++ {
		result.WriteString("\n")
	}

	return strings.TrimRight(result.String(), "\n")
}
