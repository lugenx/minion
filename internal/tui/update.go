package tui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"minion/internal/config"
)

type logMsg string
type reloadMsg struct{}
type tickActiveJobsMsg struct{}

func tickActiveJobs() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickActiveJobsMsg{}
	})
}

var logChan chan string

func listenForLogs(c chan string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return nil
		}
		msg, ok := <-c
		if !ok {
			return nil
		}
		return logMsg(msg)
	}
}

func (m *model) syncBuilderViewport() {
	w := m.width - 4
	if w < 10 { w = 10 }

	_, contentH := contentHeight(m.mainH)
	if contentH < 3 { contentH = 3 }
	m.builderViewport.Width = w
	m.builderViewport.Height = contentH

	if m.state == stateForm {
		isEditMode := true
		content := m.renderBuilderString(w, contentH, isEditMode)
		lines := strings.Split(content, "\n")

		cursorLine := -1
		for i, l := range lines {
			if strings.HasPrefix(l, "> ") || strings.Contains(l, "> ") || strings.Contains(l, "┃ ") || strings.Contains(l, "Step type") {
				cursorLine = i
				break
			}
		}

		m.builderViewport.SetContent(content)

		if cursorLine != -1 {
			if cursorLine < m.builderViewport.YOffset {
				m.builderViewport.SetYOffset(cursorLine)
			} else if cursorLine+10 >= m.builderViewport.YOffset+m.builderViewport.Height {
				m.builderViewport.SetYOffset(cursorLine + 10 - m.builderViewport.Height + 1)
			}
		}
	} else if m.state == stateDetail {
		content := m.renderBuilderString(w, contentH, false)
		m.builderViewport.SetContent(content)
	}
}

func (m *model) updateListOffset() {
	_, contentH := contentHeight(m.mainH)
	avail := contentH - 2
	if avail < 1 { avail = 1 }
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	} else if m.cursor >= m.listOffset+avail {
		m.listOffset = m.cursor - avail + 1
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	if m.state == stateForm {
		if m.confirmDelete {
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				if keyMsg.String() == "y" || keyMsg.String() == "Y" {
					row := m.builderRows[m.builderCursor]
					isStepDelete := row.Type == rowStepHeader || row.Type == rowDeleteStep
					if isStepDelete {
						steps := m.builderData.Steps
						m.builderData.Steps = append(steps[:row.StepIndex], steps[row.StepIndex+1:]...)
					} else if (row.Type == rowStepField || row.Type == rowRemoveSubItem) && row.TargetIndex != -1 {
						step := m.builderData.Steps[row.StepIndex]
						step.RemoveArrayItem(row.Field, row.TargetIndex)
					}
					m.refreshBuilderRows()
					if isStepDelete {
						deletedIdx := row.StepIndex
						if len(m.builderData.Steps) > deletedIdx {
							for i, r := range m.builderRows {
								if r.Type == rowStepHeader && r.StepIndex == deletedIdx {
									m.builderCursor = i
									break
								}
							}
						} else if len(m.builderData.Steps) > 0 {
							prevIdx := len(m.builderData.Steps) - 1
							for i, r := range m.builderRows {
								if r.Type == rowAddStep && r.StepIndex == prevIdx {
									m.builderCursor = i
									break
								}
							}
						} else if m.builderCursor >= len(m.builderRows) {
							m.builderCursor = len(m.builderRows) - 1
						}
					} else {
						if m.builderCursor >= len(m.builderRows) {
							m.builderCursor = len(m.builderRows) - 1
						}
					}
				}
				m.confirmDelete = false
				m.syncBuilderViewport()
				return m, tea.Batch(cmds...)
			}
		}

		if m.editMode {
			row := m.builderRows[m.builderCursor]
			
				if m.addStepMode {
					if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, key.NewBinding(key.WithKeys("tab"))) {
						sugs, _ := getSuggestions(true, "", "", m.textInput.Value())
						if len(sugs) > 0 {
							m.textInput.SetValue(sugs[0])
							m.textInput.SetCursor(len(sugs[0]))
						}
						m.syncBuilderViewport()
						return m, tea.Batch(cmds...)
					}

				var tiCmd tea.Cmd
				m.textInput, tiCmd = m.textInput.Update(msg)
				cmds = append(cmds, tiCmd)
				
				if keyMsg, ok := msg.(tea.KeyMsg); ok {
						if key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))) {
							val := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
							
							// Auto-select top suggestion if the input isn't exact
							sugs, isStrict := getSuggestions(true, "", "", val)
							if len(sugs) > 0 && isStrict {
								isExact := false
								for _, s := range sugs {
									if s == val {
										isExact = true
										break
									}
								}
								if !isExact {
									val = sugs[0]
								}
							}

							var newStep Step
							switch val {
							case "when": newStep = &WhenStep{}
							case "from": newStep = &FromStep{Sources: []config.Source{{}}}
							case "keep": newStep = &KeepStep{}
							case "ignore": newStep = &IgnoreStep{}
							case "do": newStep = &DoStep{}
							case "tell": newStep = &TellStep{}
							case "report": newStep = &ReportStep{}
							case "settings": newStep = &SettingsStep{Settings: config.Settings{Timeout: 15, Delay: 2}}
							}
						
						if newStep != nil {
							insertIdx := m.builderRows[m.builderCursor].StepIndex + 1
							m.builderData.Steps = append(m.builderData.Steps[:insertIdx],
								append([]Step{newStep}, m.builderData.Steps[insertIdx:]...)...)
							m.refreshBuilderRows()
							
							// Auto-focus the first field of the newly added step
							m.editMode = false
							m.addStepMode = false
							m.textInput.Blur()
							
							targetStepIndex := insertIdx
							for i, r := range m.builderRows {
								if r.Type == rowStepField && r.StepIndex == targetStepIndex {
									m.builderCursor = i
									
									// Automatically enter edit mode for this field
									if r.Field == "DoTask" {
										m.editMode = true
										m.textArea.SetValue(r.Value)
										m.textArea.Focus()
										cmds = append(cmds, textarea.Blink)
									} else {
										m.editMode = true
										m.textInput.Reset()
										m.textInput.Prompt = "> "
										m.textInput.SetValue(r.Value)
										m.textInput.Focus()
										cmds = append(cmds, textinput.Blink)
									}
									break
								}
							}
						} else {
							m.editMode = false
							m.addStepMode = false
							m.textInput.Blur()
						}
					} else if keyMsg.Type == tea.KeyEsc {
						m.editMode = false
						m.addStepMode = false
						m.textInput.Blur()
				}
				}
				m.syncBuilderViewport()
				return m, tea.Batch(cmds...)
			}
			
			if m.addSubItemField != "" {
					if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, key.NewBinding(key.WithKeys("tab"))) {
						sugs := []string{"url", "search", "minion"}
						if len(sugs) > 0 {
							m.textInput.SetValue(sugs[0])
							m.textInput.SetCursor(len(sugs[0]))
						}
						m.syncBuilderViewport()
						return m, tea.Batch(cmds...)
					}

					var tiCmd tea.Cmd
					m.textInput, tiCmd = m.textInput.Update(msg)
					cmds = append(cmds, tiCmd)

					if keyMsg, ok := msg.(tea.KeyMsg); ok {
						if key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))) {
							val := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
							if val != "url" && val != "search" && val != "minion" {
								m.syncBuilderViewport()
								return m, tea.Batch(cmds...)
							}
							step := m.builderData.Steps[m.addSubItemIdx]
							if val == "url" {
								step.AddArrayItem("FromURL")
							} else if val == "search" {
								step.AddArrayItem("FromSearch")
							} else {
								step.AddArrayItem("FromMinion")
							}
							fromStep := m.builderData.Steps[m.addSubItemIdx].(*FromStep)
							m.refreshBuilderRows()

							newTargetIdx := len(fromStep.Sources) - 1
							for i, r := range m.builderRows {
								if r.StepIndex == m.addSubItemIdx && r.TargetIndex == newTargetIdx && r.Type == rowStepField {
									m.builderCursor = i
									m.editMode = true
									m.textInput.Reset()
									m.textInput.Prompt = "> "
									m.textInput.SetValue(r.Value)
									m.textInput.Focus()
									cmds = append(cmds, textinput.Blink)
									break
								}
							}
							m.addSubItemField = ""
							m.addSubItemIdx = 0
							m.syncBuilderViewport()
							return m, tea.Batch(cmds...)
						} else if keyMsg.Type == tea.KeyEsc {
							m.editMode = false
							m.addSubItemField = ""
							m.addSubItemIdx = 0
							m.textInput.Blur()
				}
				m.syncBuilderViewport()
				return m, tea.Batch(cmds...)
				}
				}
			if row.Type == rowStepHeader && row.Field != "" {
				if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, key.NewBinding(key.WithKeys("tab"))) {
					if row.Field == "When" {
						sugs, _ := getSuggestions(false, "When", "", m.textInput.Value())
						if len(sugs) > 0 {
							m.textInput.SetValue(sugs[0])
							m.textInput.SetCursor(len(sugs[0]))
						}
						m.syncBuilderViewport()
						return m, tea.Batch(cmds...)
					}
				}
				if row.Field == "DoTask" {
					var taCmd tea.Cmd
					m.textArea, taCmd = m.textArea.Update(msg)
					cmds = append(cmds, taCmd)
					if keyMsg, ok := msg.(tea.KeyMsg); ok {
						if keyMsg.Type == tea.KeyEsc {
							val := m.textArea.Value()
							step := m.builderData.Steps[row.StepIndex]
							_ = step.UpdateField(row.Field, row.TargetIndex, val)
							m.refreshBuilderRows()
							m.editMode = false
							m.textArea.Blur()
							m.syncBuilderViewport()
							return m, tea.Batch(cmds...)
						}
					}
				} else {
					var tiCmd tea.Cmd
					m.textInput, tiCmd = m.textInput.Update(msg)
					cmds = append(cmds, tiCmd)
					if keyMsg, ok := msg.(tea.KeyMsg); ok {
						if key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))) {
							val := m.textInput.Value()
							step := m.builderData.Steps[row.StepIndex]
							_ = step.UpdateField(row.Field, row.TargetIndex, val)
							m.refreshBuilderRows()
							m.editMode = false
							m.textInput.Blur()
							m.syncBuilderViewport()
							return m, tea.Batch(cmds...)
						} else if keyMsg.Type == tea.KeyEsc {
							m.editMode = false
							m.textInput.Blur()
						}
					}
				}
				m.syncBuilderViewport()
				return m, tea.Batch(cmds...)
			}
			
				if row.Field == "DoTask" || row.Field == "DeliverPayload" || row.Field == "ReportPayload" {
					var taCmd tea.Cmd
					m.textArea, taCmd = m.textArea.Update(msg)
					cmds = append(cmds, taCmd)
					
					if keyMsg, ok := msg.(tea.KeyMsg); ok {
						if keyMsg.Type == tea.KeyEsc {
							step := m.builderData.Steps[row.StepIndex]
							_ = step.UpdateField(row.Field, row.TargetIndex, m.textArea.Value())
							m.refreshBuilderRows()
							m.editMode = false
							m.textArea.Blur()
						}
					}
				} else {
				contextType := func() string {
					if (row.Field == "TellURL" || row.Field == "ReportURL") && row.StepIndex >= 0 && row.StepIndex < len(m.builderData.Steps) && row.TargetIndex >= 0 {
						if tellStep, ok := m.builderData.Steps[row.StepIndex].(*TellStep); ok {
							if row.TargetIndex < len(tellStep.Targets) {
								for k := range tellStep.Targets[row.TargetIndex] {
									if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
										return k
									}
								}
							}
						}
						if reportStep, ok := m.builderData.Steps[row.StepIndex].(*ReportStep); ok {
							if row.TargetIndex < len(reportStep.Targets) {
								for k := range reportStep.Targets[row.TargetIndex] {
									if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
										return k
									}
								}
							}
						}
					}
					return ""
				}()

					if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, key.NewBinding(key.WithKeys("tab"))) {
						sugs, _ := getSuggestions(false, row.Field, contextType, m.textInput.Value())
						if len(sugs) > 0 {
							m.textInput.SetValue(sugs[0])
							m.textInput.SetCursor(len(sugs[0]))
						}
						m.syncBuilderViewport()
					return m, tea.Batch(cmds...)
				}

				var tiCmd tea.Cmd
				m.textInput, tiCmd = m.textInput.Update(msg)
				cmds = append(cmds, tiCmd)
				
					if keyMsg, ok := msg.(tea.KeyMsg); ok {
						if key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter"))) {
							val := m.textInput.Value()
							
								sugs, isStrict := getSuggestions(false, row.Field, contextType, val)
								if len(sugs) > 0 && isStrict {
									isExact := false
									for _, s := range sugs {
										if s == val {
											isExact = true
											break
										}
									}
									if !isExact {
										val = sugs[0]
									}
								}

							if row.StepIndex == -1 {
								if row.Field == "Name" { m.builderData.Name = val }
							} else {
								step := m.builderData.Steps[row.StepIndex]
								_ = step.UpdateField(row.Field, row.TargetIndex, val)
							}
							m.refreshBuilderRows()
							
							// Auto-jump to the next editable field within the same step
							m.editMode = false
							m.textInput.Blur()
							if row.StepIndex != -1 {
								for i := m.builderCursor + 1; i < len(m.builderRows); i++ {
									nextRow := m.builderRows[i]
									if nextRow.StepIndex == row.StepIndex && (nextRow.Type == rowStepField || nextRow.Type == rowAddSubItem || nextRow.Type == rowRemoveSubItem) && nextRow.Field != "BrowseRender" {
										m.builderCursor = i
										
										if nextRow.Type == rowAddSubItem || nextRow.Type == rowRemoveSubItem {
											// just highlight the button, don't enter edit mode
											break
										}
										
										m.editMode = true
										m.textInput.Reset()
										m.textInput.Prompt = "> "
										m.textInput.SetValue(nextRow.Value)
										m.textInput.Focus()
										cmds = append(cmds, textinput.Blink)
										break
									}
								}
							}
						} else if keyMsg.Type == tea.KeyEsc {
							m.editMode = false
							m.textInput.Blur()
						}
				}
			}
			m.syncBuilderViewport()
			return m, tea.Batch(cmds...)
		} else {
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				switch {
				case key.Matches(keyMsg, m.keys.Quit):
					return m, tea.Quit
				case key.Matches(keyMsg, m.keys.Back):
					if m.builderData != nil && m.builderData.IsNew {
						m.state = stateDashboard
						m.loadState()
					} else {
						m.state = stateDetail
						m.builderViewport.SetYOffset(0)
					}
					m.syncBuilderViewport()
					return m, nil
				case key.Matches(keyMsg, m.keys.Up):
					if m.builderCursor > 0 {
						m.builderCursor--
						for m.builderCursor > 0 && m.builderRows[m.builderCursor].Type == rowSpacer {
							m.builderCursor--
						}
					}
				case key.Matches(keyMsg, m.keys.Down):
					if m.builderCursor < len(m.builderRows)-1 {
						m.builderCursor++
						for m.builderCursor < len(m.builderRows)-1 && m.builderRows[m.builderCursor].Type == rowSpacer {
							m.builderCursor++
						}
					}
				case keyMsg.String() == "J" || keyMsg.String() == "shift+down":
					row := m.builderRows[m.builderCursor]
					if row.Type == rowStepHeader && row.StepIndex < len(m.builderData.Steps)-1 {
						m.builderData.Steps[row.StepIndex], m.builderData.Steps[row.StepIndex+1] = m.builderData.Steps[row.StepIndex+1], m.builderData.Steps[row.StepIndex]
						m.refreshBuilderRows()
						// adjust cursor to follow
						for i, r := range m.builderRows {
							if r.Type == rowStepHeader && r.StepIndex == row.StepIndex+1 {
								m.builderCursor = i
								break
							}
						}
					}
				case keyMsg.String() == "K" || keyMsg.String() == "shift+up":
					row := m.builderRows[m.builderCursor]
					if row.Type == rowStepHeader && row.StepIndex > 0 {
						m.builderData.Steps[row.StepIndex], m.builderData.Steps[row.StepIndex-1] = m.builderData.Steps[row.StepIndex-1], m.builderData.Steps[row.StepIndex]
						m.refreshBuilderRows()
						for i, r := range m.builderRows {
							if r.Type == rowStepHeader && r.StepIndex == row.StepIndex-1 {
								m.builderCursor = i
								break
							}
						}
					}
					case keyMsg.String() == "x" || keyMsg.String() == "delete":
						row := m.builderRows[m.builderCursor]
						if row.Type == rowStepHeader || row.Type == rowDeleteStep || ((row.Type == rowStepField || row.Type == rowRemoveSubItem) && row.TargetIndex != -1) {
							m.confirmDelete = true
						}
				case keyMsg.String() == "a":
					m.builderCursor = len(m.builderRows) - 1 // move cursor to Add Step
					m.addStepMode = true
					m.editMode = true
					m.textInput.Reset()
					m.textInput.Prompt = "Block type (when/from/do/keep/ignore/tell/settings): "
					m.textInput.Focus()
					cmds = append(cmds, textinput.Blink)
				case keyMsg.String() == "s":
					_ = saveBuilder(m.builderData)
					m.dirty = false
					m.syncBuilderViewport()
				case keyMsg.String() == "enter" || keyMsg.String() == "e":
					row := m.builderRows[m.builderCursor]
					
					if row.Type == rowAddStep {
						m.addStepMode = true
						m.editMode = true
						m.textInput.Reset()
						m.textInput.Prompt = "Block type (when/from/do/keep/ignore/tell/settings): "
						m.textInput.Focus()
						cmds = append(cmds, textinput.Blink)
					} else if row.Type == rowEnabled {
						m.builderData.Enabled = !m.builderData.Enabled
						m.refreshBuilderRows()
				} else if row.Field == "BrowseRender" {
					step := m.builderData.Steps[row.StepIndex]
					_ = step.UpdateField("BrowseRenderToggle", row.TargetIndex, "")
					m.refreshBuilderRows()
				} else if row.Field == "DeliverMarkdown" {
					step := m.builderData.Steps[row.StepIndex]
					_ = step.UpdateField("DeliverMarkdownToggle", row.TargetIndex, "")
					m.refreshBuilderRows()
				} else if row.Field == "ReportMarkdown" {
					step := m.builderData.Steps[row.StepIndex]
					_ = step.UpdateField("ReportMarkdownToggle", row.TargetIndex, "")
					m.refreshBuilderRows()
					} else if row.Type == rowRemoveSubItem || row.Type == rowDeleteStep {
						m.confirmDelete = true
				} else if row.Type == rowAddSubItem {
					if row.Field == "From" {
						m.addSubItemIdx = row.StepIndex
						m.addSubItemField = "From"
						m.editMode = true
						m.textInput.Reset()
						m.textInput.Prompt = "Source type (url/search): "
						m.textInput.Focus()
						cmds = append(cmds, textinput.Blink)
					} else {
						step := m.builderData.Steps[row.StepIndex]
						step.AddArrayItem(row.Field)
						m.refreshBuilderRows()

						m.editMode = false
						m.textInput.Blur()

						if m.builderCursor > 0 {

							targetIndexToFind := -1
							var targetField string

							if row.Field == "BrowseAddTarget" {
								targetField = "BrowseURL"
							} else if row.Field == "SearchAddQuery" {
								targetField = "SearchQuery"
							} else if row.Field == "FilterAddKeep" {
								targetField = "FilterKeep"
							} else if row.Field == "FilterAddDrop" {
								targetField = "FilterDrop"
							} else if row.Field == "Tell" {
								targetField = "TellURL"
							} else if row.Field == "Report" {
								targetField = "ReportURL"
							}

							if targetField != "" {
								for i := len(m.builderRows) - 1; i >= 0; i-- {
									r := m.builderRows[i]
									if r.StepIndex == row.StepIndex && r.Field == targetField {
										if r.TargetIndex > targetIndexToFind {
											targetIndexToFind = r.TargetIndex
											m.builderCursor = i
										}
									}
								}
							}

							targetRow := m.builderRows[m.builderCursor]
							if targetRow.Type == rowStepField {
								m.editMode = true
								m.textInput.Reset()
								m.textInput.Prompt = "> "
								m.textInput.SetValue(targetRow.Value)
								m.textInput.Focus()
								cmds = append(cmds, textinput.Blink)
							}
						}
					}
					} else if row.Type == rowStepHeader {
						if row.Field != "" {
							if row.Field == "DoTask" {
								m.editMode = true
								m.textArea.SetValue(row.Value)
								m.textArea.Focus()
								cmds = append(cmds, textarea.Blink)
							} else {
								m.editMode = true
								m.textInput.Reset()
								if row.Field == "KeepWords" || row.Field == "IgnoreWords" {
									m.textInput.Prompt = "(comma separated)> "
								} else {
									m.textInput.Prompt = "> "
								}
								m.textInput.SetValue(row.Value)
								m.textInput.Focus()
								cmds = append(cmds, textinput.Blink)
							}
						} else {
							// Hitting enter on a step header: auto-jump to its first field
							for i, r := range m.builderRows {
								if r.Type == rowStepField && r.StepIndex == row.StepIndex {
									m.builderCursor = i
									if r.Field == "DoTask" {
										m.editMode = true
										m.textArea.SetValue(r.Value)
										m.textArea.Focus()
										cmds = append(cmds, textarea.Blink)
									} else {
										m.editMode = true
										m.textInput.Reset()
										m.textInput.Prompt = "> "
										m.textInput.SetValue(r.Value)
										m.textInput.Focus()
										cmds = append(cmds, textinput.Blink)
									}
									break
								}
							}
						}
					} else if row.Type == rowStepField || row.Type == rowName {
						if row.Field == "DoTask" || row.Field == "DeliverPayload" || row.Field == "ReportPayload" {
							m.editMode = true
							m.textArea.SetValue(row.Value)
							m.textArea.Focus()
							cmds = append(cmds, textarea.Blink)
						} else {
							m.editMode = true
							m.textInput.Reset()
							m.textInput.Prompt = "> "
							if row.StepIndex == -1 {
								if row.Field == "Name" { m.textInput.SetValue(m.builderData.Name) }
							} else {
								m.textInput.SetValue(row.Value)
							}
							m.textInput.Focus()
							cmds = append(cmds, textinput.Blink)
						}
					}
				}
			}

			m.syncBuilderViewport()
			return m, tea.Batch(cmds...)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width - 4
		if m.help.Width < 10 { m.help.Width = 10 }
		
		headerH := lipgloss.Height(m.renderHeader(m.width - 2))
		footerH := lipgloss.Height(m.renderFooter(m.width - 2))
		m.mainH = m.height - headerH - footerH
		if m.mainH < 5 { m.mainH = 5 }

		vpWidth := m.width - 6
		if vpWidth < 10 {
			vpWidth = 10
		}
		
		vpHeight := m.mainH - 3
		if vpHeight < 5 {
			vpHeight = 5
		}
		
		m.textArea.SetWidth(vpWidth)
		m.textArea.SetHeight(vpHeight)
		
		if m.logViewport.Width == 0 {
			m.logViewport = viewport.New(vpWidth, vpHeight)
		} else {
			m.logViewport.Width = vpWidth
			m.logViewport.Height = vpHeight
		}

		if m.state == stateDetail || m.state == stateForm {
			m.syncBuilderViewport()
		}

		case tea.KeyMsg:
			switch {
					case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit

					case key.Matches(msg, m.keys.Back):
				if m.state == stateLogs {
					m.tailing = false
					m.state = stateDashboard
					m.loadState()
					m.syncBuilderViewport()
				}
			}

		if m.state == stateDashboard {
			if m.confirmDelete {
				if msg.String() == "y" || msg.String() == "Y" {
					if len(m.minions) > 0 {
						target := m.minions[m.cursor].Filename
						path := filepath.Join(config.MinionsDir, target)
						_ = os.Remove(path)
						m.loadState()
						
						if m.cursor >= len(m.minions) {
							m.cursor = len(m.minions) - 1
							if m.cursor < 0 { m.cursor = 0 }
						}
						m.updateListOffset()
					}
				}
				m.confirmDelete = false
				m.syncBuilderViewport()
				return m, tea.Batch(cmds...)
			}

			switch {
				case key.Matches(msg, m.keys.Up):
					if m.cursor > 0 {
						m.cursor--
						m.updateListOffset()
						m.builderViewport.SetYOffset(0)
						m.syncBuilderViewport()
					}
				case key.Matches(msg, m.keys.Down):
					if m.cursor < len(m.minions)-1 {
						m.cursor++
						m.updateListOffset()
						m.builderViewport.SetYOffset(0)
						m.syncBuilderViewport()
					}
					case key.Matches(msg, m.keys.Toggle):
						if len(m.minions) > 0 {
							selected := m.minions[m.cursor]
							if selected.Enabled != nil && !*selected.Enabled {
								return m, nil
							}
							if m.db != nil {
								currentState := m.db.GetMinionStatus(selected.Filename)
								newState := !currentState
								_ = m.db.SetMinionStatus(selected.Filename, newState)
								
								if newState {
									m.upCount++
									if !m.daemonRunning {
										c := exec.Command(os.Args[0], "run", "-d")
										_ = c.Run()
										m.daemonRunning = true
									}
								} else {
									m.upCount--
									if m.upCount <= 0 && m.daemonRunning {
										c := exec.Command(os.Args[0], "down")
										_ = c.Run()
										m.daemonRunning = false
									}
								}
							}
							return m, nil
						}
				case key.Matches(msg, m.keys.Enter):
					if len(m.minions) > 0 {
						m.state = stateDetail
						m.builderViewport.SetYOffset(0)
						m.syncBuilderViewport()
					}
					case key.Matches(msg, m.keys.Env):
						envData, err := os.ReadFile(config.EnvPath)
						if err == nil {
							m.textArea.SetValue(string(envData))
						} else {
							m.textArea.SetValue("")
						}
						m.textArea.Focus()
						cmds = append(cmds, textarea.Blink)
						m.state = stateEnv
						m.syncBuilderViewport()
						return m, tea.Batch(cmds...)
					case key.Matches(msg, m.keys.New):
					m.builderData = initBuilderData(nil)
					m.refreshBuilderRows()
					m.dirty = false
					m.builderCursor = 0
					m.editMode = true
					m.textInput.Reset()
					m.textInput.Prompt = "> "
					if len(m.builderRows) > 0 { m.textInput.SetValue(m.builderRows[0].Value) }
					m.textInput.Focus()
					cmds = append(cmds, textinput.Blink)
					m.state = stateForm
					m.syncBuilderViewport()
					return m, tea.Batch(cmds...)
				case key.Matches(msg, m.keys.Delete):
					if len(m.minions) > 0 {
						m.confirmDelete = true
						m.syncBuilderViewport()
						return m, tea.Batch(cmds...)
					}
					case key.Matches(msg, m.keys.Run):
						if len(m.minions) > 0 {
							selected := m.minions[m.cursor]
							
							if !m.activeJobs[selected.Filename] {
								if m.db != nil {
									_ = m.db.QueueRun(selected.Filename)

									if !m.daemonRunning {
										c := exec.Command(os.Args[0], "run", "-d")
										_ = c.Run()
										m.daemonRunning = true
									}
								}
							}
							
							m.state = stateLogs
							m.logContent = ""
							m.logViewport.SetContent("")
							m.tailing = true
							
							logChan = make(chan string, 100)
							ctx := context.Background()
							
							cmds = append(cmds, tailLogCmd(ctx, selected, logChan), listenForLogs(logChan))
							return m, tea.Batch(cmds...)
						}
					case key.Matches(msg, m.keys.Stop):
						if len(m.minions) > 0 {
							selected := m.minions[m.cursor]
							if m.activeJobs[selected.Filename] && m.db != nil {
								fn := selected.Filename
								go func() { _ = m.db.QueueAbort(fn) }()
							}
							return m, nil
						}
					case key.Matches(msg, m.keys.Log):
						if len(m.minions) > 0 {
							m.state = stateLogs
							m.logContent = ""
							m.logViewport.SetContent("")
							m.tailing = true
							
							logChan = make(chan string, 100)
							ctx := context.Background()
							
							selected := m.minions[m.cursor]
							cmds = append(cmds, tailLogCmd(ctx, selected, logChan), listenForLogs(logChan))
							return m, tea.Batch(cmds...)
						}
					case key.Matches(msg, m.keys.Edit):
						if len(m.minions) > 0 {
							selected := m.minions[m.cursor]
							rawMinion := loadRawMinionForEditing(selected.Filename)
							if rawMinion == nil {
								rawMinion = selected // Fallback
							}
							
							m.builderData = initBuilderData(rawMinion)
							m.refreshBuilderRows()
							m.dirty = false
							m.builderCursor = 0
							m.editMode = false
							m.state = stateForm
							m.syncBuilderViewport()
							return m, nil
						}
					}
			} else if m.state == stateEnv {
			var taCmd tea.Cmd
			m.textArea, taCmd = m.textArea.Update(msg)
			cmds = append(cmds, taCmd)
			
			if key.Matches(msg, m.keys.Back) {
				// Save env
				_ = os.WriteFile(config.EnvPath, []byte(m.textArea.Value()), 0644)
				_ = godotenv.Overload(config.EnvPath) // Hot reload without restart
				
				m.textArea.Blur()
				m.state = stateDashboard
					m.loadState()
					m.syncBuilderViewport()
				}
					} else if m.state == stateLogs {
						m.logViewport, cmd = m.logViewport.Update(msg)
						cmds = append(cmds, cmd)
	
						// Allow pressing 's' to stop a minion even while viewing logs!
						if key.Matches(msg, m.keys.Stop) {
							if len(m.minions) > 0 {
								selected := m.minions[m.cursor]
								if m.activeJobs[selected.Filename] && m.db != nil {
									fn := selected.Filename
									go func() { _ = m.db.QueueAbort(fn) }()
									// Inject a bright orange line into the logs so user knows it worked
									m.logContent += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[System] Stopping...") + "\n"
									m.logViewport.SetContent(m.logContent)
									m.logViewport.GotoBottom()
								}
						}
					}
					} else if m.state == stateDetail {
						switch {
						case key.Matches(msg, m.keys.Back):
							m.state = stateDashboard
							m.loadState()
							m.syncBuilderViewport()
						case key.Matches(msg, m.keys.Up):
							m.builderViewport.LineUp(1)
						case key.Matches(msg, m.keys.Down):
							m.builderViewport.LineDown(1)
						case key.Matches(msg, m.keys.Toggle):
							if len(m.minions) > 0 {
								selected := m.minions[m.cursor]
								if selected.Enabled != nil && !*selected.Enabled {
									return m, nil
								}
								if m.db != nil {
									currentState := m.db.GetMinionStatus(selected.Filename)
									newState := !currentState
									_ = m.db.SetMinionStatus(selected.Filename, newState)
									if newState {
										m.upCount++
										if !m.daemonRunning {
											c := exec.Command(os.Args[0], "run", "-d")
											_ = c.Run()
											m.daemonRunning = true
										}
									} else {
										m.upCount--
										if m.upCount <= 0 && m.daemonRunning {
											c := exec.Command(os.Args[0], "down")
											_ = c.Run()
											m.daemonRunning = false
										}
									}
								}
								return m, nil
							}
						case key.Matches(msg, m.keys.Run):
							if len(m.minions) > 0 {
								selected := m.minions[m.cursor]
								if !m.activeJobs[selected.Filename] {
									if m.db != nil {
										_ = m.db.QueueRun(selected.Filename)
										if !m.daemonRunning {
											c := exec.Command(os.Args[0], "run", "-d")
											_ = c.Run()
											m.daemonRunning = true
										}
									}
								}
								m.state = stateLogs
								m.logContent = ""
								m.logViewport.SetContent("")
								m.tailing = true
								logChan = make(chan string, 100)
								ctx := context.Background()
								cmds = append(cmds, tailLogCmd(ctx, selected, logChan), listenForLogs(logChan))
								return m, tea.Batch(cmds...)
							}
						case key.Matches(msg, m.keys.Stop):
							if len(m.minions) > 0 {
								selected := m.minions[m.cursor]
								if m.activeJobs[selected.Filename] && m.db != nil {
									fn := selected.Filename
									go func() { _ = m.db.QueueAbort(fn) }()
								}
								return m, nil
							}
						case key.Matches(msg, m.keys.Log):
							if len(m.minions) > 0 {
								m.state = stateLogs
								m.logContent = ""
								m.logViewport.SetContent("")
								m.tailing = true
								logChan = make(chan string, 100)
								ctx := context.Background()
								selected := m.minions[m.cursor]
								cmds = append(cmds, tailLogCmd(ctx, selected, logChan), listenForLogs(logChan))
								return m, tea.Batch(cmds...)
							}
						case key.Matches(msg, m.keys.Edit):
							if len(m.minions) > 0 {
								selected := m.minions[m.cursor]
								rawMinion := loadRawMinionForEditing(selected.Filename)
								if rawMinion == nil {
									rawMinion = selected
								}
							m.builderData = initBuilderData(rawMinion)
							m.refreshBuilderRows()
							m.dirty = false
							m.builderCursor = 0
								m.editMode = false
								m.state = stateForm
								m.syncBuilderViewport()
								return m, nil
							}
						}
					}
	
		case logMsg:
			// logContent replaces testLog
			m.logContent += string(msg) + "\n"
			m.logViewport.SetContent(m.logContent)
			m.logViewport.GotoBottom()
			cmds = append(cmds, listenForLogs(logChan))

	case reloadMsg:
		m.loadState()
		
	case tickActiveJobsMsg:
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
		}
		
		if _, err := os.Stat(config.PIDPath); err == nil {
			m.daemonRunning = true
		} else {
			m.daemonRunning = false
		}
		
		return m, tickActiveJobs()
	}

	m.logSpinner, cmd = m.logSpinner.Update(msg)
	cmds = append(cmds, cmd)

	if m.state == stateForm || m.state == stateDashboard || m.state == stateDetail {
		var vpCmd tea.Cmd
		m.builderViewport, vpCmd = m.builderViewport.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

	func tailLogCmd(ctx context.Context, m *config.MinionConfig, ch chan string) tea.Cmd {
	return func() tea.Msg {
		go func() {
			defer close(ch)
			
			displayFile := strings.TrimSuffix(m.Filename, ".yaml")
			displayFile = strings.TrimSuffix(displayFile, ".yml")
			targetLog := filepath.Join(config.LogsDir, displayFile+".log")
			
			if _, err := os.Stat(targetLog); os.IsNotExist(err) {
				ch <- fmt.Sprintf("Log file not found at %s", targetLog)
				ch <- "This minion has not run yet."
				return
			}
			
			cmd := exec.CommandContext(ctx, "tail", "-n", "100", "-f", targetLog)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				ch <- fmt.Sprintf("Error creating stdout pipe: %v", err)
				return
			}
			
			if err := cmd.Start(); err != nil {
				ch <- fmt.Sprintf("Error starting tail: %v", err)
				return
			}
			
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				ch <- scanner.Text()
			}
			
			_ = cmd.Wait()
		}()
		return nil
	}
}
