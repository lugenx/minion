package tui

import (
	"fmt"
	"strconv"
	"strings"
)

type Step interface {
	Type() StepType
	FromYAML(stepMap map[string]interface{})
	ToYAML() map[string]interface{}
	GetRows(stepIndex int) []builderRow
	UpdateField(field string, targetIndex int, value string) error
	AddArrayItem(field string)
	RemoveArrayItem(field string, index int)
}

type ScheduleStep struct {
	Schedule string
}

func (s *ScheduleStep) Type() StepType { return StepSchedule }

func (s *ScheduleStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["schedule"]; ok {
		s.Schedule = fmt.Sprintf("%v", val)
	}
}

func (s *ScheduleStep) ToYAML() map[string]interface{} {
	if strings.TrimSpace(s.Schedule) != "" {
		return map[string]interface{}{"schedule": strings.TrimSpace(s.Schedule)}
	}
	return nil
}

func (s *ScheduleStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "Schedule", Label: "value", Value: s.Schedule},
	}
}

func (s *ScheduleStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "Schedule" {
		s.Schedule = value
	}
	return nil
}

func (s *ScheduleStep) AddArrayItem(field string) {}
func (s *ScheduleStep) RemoveArrayItem(field string, index int) {}

type SearchStep struct {
	Queries []string
	Limit   string
}

func (s *SearchStep) Type() StepType { return StepSearch }

func (s *SearchStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["search"]; ok {
		if searches, ok := val.([]interface{}); ok {
			for _, search := range searches {
				s.Queries = append(s.Queries, fmt.Sprintf("%v", search))
			}
		} else if searchStr, ok := val.(string); ok {
			s.Queries = append(s.Queries, searchStr)
		}
	}
	if limit, ok := stepMap["limit"]; ok {
		s.Limit = fmt.Sprintf("%v", limit)
	}
}

func (s *SearchStep) ToYAML() map[string]interface{} {
	var validQueries []string
	for _, sq := range s.Queries {
		clean := strings.TrimSpace(sq)
		if clean != "" {
			validQueries = append(validQueries, clean)
		}
	}
	if len(validQueries) > 0 {
		mStep := map[string]interface{}{"search": validQueries}
		if s.Limit != "" {
			if limit, err := strconv.Atoi(strings.TrimSpace(s.Limit)); err == nil {
				mStep["limit"] = limit
			}
		}
		return mStep
	}
	return nil
}

func (s *SearchStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	if len(s.Queries) == 0 {
		s.Queries = []string{""}
	}
	for i, q := range s.Queries {
		label := "query"
		if i > 0 { label = "" }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "SearchQuery", Label: label, Value: q})
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: i, Field: "SearchRemoveQuery", Label: "[ - Remove Query ]"})
		if i < len(s.Queries)-1 {
			rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
		}
	}
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "SearchAddQuery", Label: "[ + Add Query ]"})
	
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex})
	
	rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "SearchLimit", Label: "limit", Value: s.Limit})
	return rows
}

func (s *SearchStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "SearchQuery" && targetIndex >= 0 && targetIndex < len(s.Queries) {
		s.Queries[targetIndex] = value
	} else if field == "SearchLimit" {
		s.Limit = value
	}
	return nil
}

func (s *SearchStep) AddArrayItem(field string) {
	if field == "SearchAddQuery" {
		s.Queries = append(s.Queries, "")
	}
}

func (s *SearchStep) RemoveArrayItem(field string, index int) {
	if (field == "SearchQuery" || field == "SearchRemoveQuery") && index >= 0 && index < len(s.Queries) {
		s.Queries = append(s.Queries[:index], s.Queries[index+1:]...)
	}
}

type BrowseStep struct {
	Targets []BrowseTarget
}

func (s *BrowseStep) Type() StepType { return StepBrowse }

func (s *BrowseStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["browse"]; ok {
		if browses, ok := val.([]interface{}); ok {
			for _, b := range browses {
				target := BrowseTarget{}
				if bMap, ok := b.(map[string]interface{}); ok {
					if u, ok := bMap["url"]; ok { target.URL = fmt.Sprintf("%v", u) }
					if match, ok := bMap["match"]; ok { target.Match = fmt.Sprintf("%v", match) }
					if render, ok := bMap["render"]; ok {
						if bBool, ok := render.(bool); ok { target.Render = bBool }
					}
				}
				s.Targets = append(s.Targets, target)
			}
		}
	}
}

func (s *BrowseStep) ToYAML() map[string]interface{} {
	if len(s.Targets) > 0 {
		var bArr []map[string]interface{}
		for _, t := range s.Targets {
			if strings.TrimSpace(t.URL) == "" { continue }
			bMap := map[string]interface{}{"url": strings.TrimSpace(t.URL)}
			if strings.TrimSpace(t.Match) != "" {
				bMap["match"] = strings.TrimSpace(t.Match)
			}
			if t.Render {
				bMap["render"] = true
			}
			bArr = append(bArr, bMap)
		}
		if len(bArr) > 0 {
			return map[string]interface{}{"browse": bArr}
		}
	}
	return nil
}

func (s *BrowseStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	if len(s.Targets) == 0 {
		s.Targets = []BrowseTarget{{}}
	}
	for tIdx, target := range s.Targets {
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: tIdx, Field: "BrowseURL", Label: "url", Value: target.URL})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: tIdx, Field: "BrowseMatch", Label: "match", Value: target.Match})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: tIdx, Field: "BrowseRender", Label: "render", Value: fmt.Sprintf("%t", target.Render)})
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: tIdx, Field: "BrowseRemoveTarget", Label: "[ - Remove Target ]"})
		if tIdx < len(s.Targets)-1 {
			rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex})
		}
	}
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "BrowseAddTarget", Label: "[ + Add Target ]"})
	return rows
}

func (s *BrowseStep) UpdateField(field string, targetIndex int, value string) error {
	if targetIndex >= 0 && targetIndex < len(s.Targets) {
		if field == "BrowseURL" {
			s.Targets[targetIndex].URL = value
		} else if field == "BrowseMatch" {
			s.Targets[targetIndex].Match = value
		} else if field == "BrowseRenderToggle" {
			s.Targets[targetIndex].Render = !s.Targets[targetIndex].Render
		}
	}
	return nil
}

func (s *BrowseStep) AddArrayItem(field string) {
	if field == "BrowseAddTarget" {
		s.Targets = append(s.Targets, BrowseTarget{})
	}
}

func (s *BrowseStep) RemoveArrayItem(field string, index int) {
	if (field == "BrowseURL" || field == "BrowseMatch" || field == "BrowseRender" || field == "BrowseRemoveTarget") && index >= 0 && index < len(s.Targets) {
		s.Targets = append(s.Targets[:index], s.Targets[index+1:]...)
	}
}

type FilterStep struct {
	Keep []string
	Drop []string
}

func (s *FilterStep) Type() StepType { return StepFilter }

func (s *FilterStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["filter"]; ok {
		if fMap, ok := val.(map[string]interface{}); ok {
			if keep, ok := fMap["keep"].([]interface{}); ok {
				for _, k := range keep { s.Keep = append(s.Keep, fmt.Sprintf("%v", k)) }
			} else if keepStr, ok := fMap["keep"].(string); ok {
				s.Keep = append(s.Keep, keepStr)
			}
			
			if drop, ok := fMap["drop"].([]interface{}); ok {
				for _, d := range drop { s.Drop = append(s.Drop, fmt.Sprintf("%v", d)) }
			} else if dropStr, ok := fMap["drop"].(string); ok {
				s.Drop = append(s.Drop, dropStr)
			}
		}
	}
}

func (s *FilterStep) ToYAML() map[string]interface{} {
	var validKeep []string
	for _, k := range s.Keep {
		clean := strings.TrimSpace(k)
		if clean != "" { validKeep = append(validKeep, clean) }
	}
	
	var validDrop []string
	for _, d := range s.Drop {
		clean := strings.TrimSpace(d)
		if clean != "" { validDrop = append(validDrop, clean) }
	}
	
	if len(validKeep) > 0 || len(validDrop) > 0 {
		fMap := map[string]interface{}{}
		if len(validKeep) > 0 { fMap["keep"] = validKeep }
		if len(validDrop) > 0 { fMap["drop"] = validDrop }
		return map[string]interface{}{"filter": fMap}
	}
	return nil
}

func (s *FilterStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	
	if len(s.Keep) == 0 { s.Keep = []string{""} }
	for i, k := range s.Keep {
		label := "keep"
		if i > 0 { label = "" }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FilterKeep", Label: label, Value: k})
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: i, Field: "FilterRemoveKeep", Label: "[ - Remove Keep ]"})
		if i < len(s.Keep)-1 {
			rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
		}
	}
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "FilterAddKeep", Label: "[ + Add Keep ]"})
	
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex})
	
	if len(s.Drop) == 0 { s.Drop = []string{""} }
	for i, d := range s.Drop {
		label := "drop"
		if i > 0 { label = "" }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FilterDrop", Label: label, Value: d})
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: i, Field: "FilterRemoveDrop", Label: "[ - Remove Drop ]"})
		if i < len(s.Drop)-1 {
			rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
		}
	}
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "FilterAddDrop", Label: "[ + Add Drop ]"})
	
	return rows
}

func (s *FilterStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "FilterKeep" && targetIndex >= 0 && targetIndex < len(s.Keep) {
		s.Keep[targetIndex] = value
	} else if field == "FilterDrop" && targetIndex >= 0 && targetIndex < len(s.Drop) {
		s.Drop[targetIndex] = value
	}
	return nil
}

func (s *FilterStep) AddArrayItem(field string) {
	if field == "FilterAddKeep" {
		s.Keep = append(s.Keep, "")
	} else if field == "FilterAddDrop" {
		s.Drop = append(s.Drop, "")
	}
}

func (s *FilterStep) RemoveArrayItem(field string, index int) {
	if (field == "FilterKeep" || field == "FilterRemoveKeep") && index >= 0 && index < len(s.Keep) {
		s.Keep = append(s.Keep[:index], s.Keep[index+1:]...)
	} else if (field == "FilterDrop" || field == "FilterRemoveDrop") && index >= 0 && index < len(s.Drop) {
		s.Drop = append(s.Drop[:index], s.Drop[index+1:]...)
	}
}

type ScrapeStep struct {
	Timeout string
	Delay   string
}

func (s *ScrapeStep) Type() StepType { return StepScrape }

func (s *ScrapeStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["scrape"]; ok {
		if sMap, ok := val.(map[string]interface{}); ok {
			if t, ok := sMap["timeout"]; ok { s.Timeout = fmt.Sprintf("%v", t) }
			if d, ok := sMap["delay"]; ok { s.Delay = fmt.Sprintf("%v", d) }
		} else if sMap, ok := val.(map[interface{}]interface{}); ok {
			if t, ok := sMap["timeout"]; ok { s.Timeout = fmt.Sprintf("%v", t) }
			if d, ok := sMap["delay"]; ok { s.Delay = fmt.Sprintf("%v", d) }
		} else {
			s.Timeout = "15"
			s.Delay = "2"
		}
	}
}

func (s *ScrapeStep) ToYAML() map[string]interface{} {
	sMap := map[string]interface{}{}
	if s.Timeout != "" {
		if to, err := strconv.Atoi(strings.TrimSpace(s.Timeout)); err == nil {
			sMap["timeout"] = to
		}
	}
	if s.Delay != "" {
		if delay, err := strconv.Atoi(strings.TrimSpace(s.Delay)); err == nil {
			sMap["delay"] = delay
		}
	}
	if len(sMap) > 0 {
		return map[string]interface{}{"scrape": sMap}
	}
	return map[string]interface{}{"scrape": nil}
}

func (s *ScrapeStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ScrapeTimeout", Label: "timeout", Value: s.Timeout},
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ScrapeDelay", Label: "delay", Value: s.Delay},
	}
}

func (s *ScrapeStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "ScrapeTimeout" {
		s.Timeout = value
	} else if field == "ScrapeDelay" {
		s.Delay = value
	}
	return nil
}

func (s *ScrapeStep) AddArrayItem(field string) {}
func (s *ScrapeStep) RemoveArrayItem(field string, index int) {}

type StudyStep struct {
	Task string
}

func (s *StudyStep) Type() StepType { return StepStudy }

func (s *StudyStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["study"]; ok {
		if sMap, ok := val.(map[string]interface{}); ok {
			if t, ok := sMap["task"]; ok { s.Task = fmt.Sprintf("%v", t) }
		}
	}
}

func (s *StudyStep) ToYAML() map[string]interface{} {
	if strings.TrimSpace(s.Task) != "" {
		return map[string]interface{}{
			"study": map[string]interface{}{
				"task": strings.TrimSpace(s.Task),
			},
		}
	}
	return nil
}

func (s *StudyStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "StudyTask", Label: "task", Value: s.Task},
	}
}

func (s *StudyStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "StudyTask" {
		s.Task = value
	}
	return nil
}

func (s *StudyStep) AddArrayItem(field string) {}
func (s *StudyStep) RemoveArrayItem(field string, index int) {}

type ActionTarget struct {
	Type            string
	Target          string
	Username        string
	Password        string
	Method          string
	Headers         string
	PayloadTemplate string
}

type DeliverStep struct {
	Targets []ActionTarget
}

func (s *DeliverStep) Type() StepType { return StepDeliver }

func (s *DeliverStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["deliver"]; ok {
		if dArr, ok := val.([]interface{}); ok {
			for _, d := range dArr {
				if dMap, ok := d.(map[string]interface{}); ok {
					var at ActionTarget
					for k, v := range dMap {
						if k == "basic_auth" {
							if authMap, ok := v.(map[string]interface{}); ok {
								if u, ok := authMap["username"].(string); ok { at.Username = u }
								if p, ok := authMap["password"].(string); ok { at.Password = p }
							}
						} else if k == "method" {
							if m, ok := v.(string); ok { at.Method = m }
						} else if k == "headers" {
							if hMap, ok := v.(map[string]interface{}); ok {
								var hStrs []string
								for hk, hv := range hMap {
									hStrs = append(hStrs, fmt.Sprintf("%s: %v", hk, hv))
								}
								at.Headers = strings.Join(hStrs, ", ")
							}
						} else if k == "payload_template" {
							if p, ok := v.(string); ok { at.PayloadTemplate = p }
						} else {
							if str, ok := v.(string); ok {
								at.Type = k
								at.Target = str
							}
						}
					}
					s.Targets = append(s.Targets, at)
				}
			}
		}
	}
}

func (s *DeliverStep) ToYAML() map[string]interface{} {
	var validTargets []map[string]interface{}
	for _, t := range s.Targets {
		cleanType := strings.TrimSpace(t.Type)
		cleanTarget := strings.TrimSpace(t.Target)
		if cleanType != "" && cleanTarget != "" {
			tMap := map[string]interface{}{cleanType: cleanTarget}
			
			if cleanType == "ntfy" {
				if t.Username != "" || t.Password != "" {
					tMap["basic_auth"] = map[string]interface{}{
						"username": t.Username,
						"password": t.Password,
					}
				}
			} else if cleanType == "http_request" {
				if t.Method != "" {
					tMap["method"] = t.Method
				}
				if t.Headers != "" {
					headersMap := make(map[string]interface{})
					for _, h := range strings.Split(t.Headers, ",") {
						parts := strings.SplitN(h, ":", 2)
						if len(parts) == 2 {
							headersMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
						}
					}
					if len(headersMap) > 0 {
						tMap["headers"] = headersMap
					}
				}
				if t.PayloadTemplate != "" {
					tMap["payload_template"] = t.PayloadTemplate
				}
			}
			
			validTargets = append(validTargets, tMap)
		}
	}
	
	if len(validTargets) > 0 {
		return map[string]interface{}{"deliver": validTargets}
	}
	return nil
}

func (s *DeliverStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	if len(s.Targets) == 0 {
		s.Targets = []ActionTarget{{}}
	}
	for i, t := range s.Targets {
		typeLabel := "type"
		if i > 0 { typeLabel = "" }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverType", Label: typeLabel, Value: t.Type})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverTarget", Label: "target", Value: t.Target})
		
		if t.Type == "ntfy" {
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverUsername", Label: "username", Value: t.Username})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverPassword", Label: "password", Value: t.Password})
		} else if t.Type == "http_request" {
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverMethod", Label: "method", Value: t.Method})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverHeaders", Label: "headers", Value: t.Headers})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverPayload", Label: "payload", Value: t.PayloadTemplate})
		}
		
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: i, Field: "DeliverRemoveTarget", Label: "[ - Remove Target ]"})
		if i < len(s.Targets)-1 {
			rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
		}
	}
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverAddTarget", Label: "[ + Add Target ]"})
	return rows
}

func (s *DeliverStep) UpdateField(field string, targetIndex int, value string) error {
	if targetIndex >= 0 && targetIndex < len(s.Targets) {
		if field == "DeliverType" {
			s.Targets[targetIndex].Type = value
		} else if field == "DeliverTarget" {
			s.Targets[targetIndex].Target = value
		} else if field == "DeliverUsername" {
			s.Targets[targetIndex].Username = value
		} else if field == "DeliverPassword" {
			s.Targets[targetIndex].Password = value
		} else if field == "DeliverMethod" {
			s.Targets[targetIndex].Method = value
		} else if field == "DeliverHeaders" {
			s.Targets[targetIndex].Headers = value
		} else if field == "DeliverPayload" {
			s.Targets[targetIndex].PayloadTemplate = value
		}
	}
	return nil
}

func (s *DeliverStep) AddArrayItem(field string) {
	if field == "DeliverAddTarget" {
		s.Targets = append(s.Targets, ActionTarget{})
	}
}

func (s *DeliverStep) RemoveArrayItem(field string, index int) {
	if (field == "DeliverType" || field == "DeliverTarget" || field == "DeliverRemoveTarget") && index >= 0 && index < len(s.Targets) {
		s.Targets = append(s.Targets[:index], s.Targets[index+1:]...)
	}
}

type ReceiveStep struct {
	TargetURL string
}

func (s *ReceiveStep) Type() StepType { return StepReceive }

func (s *ReceiveStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["receive"]; ok {
		s.TargetURL = fmt.Sprintf("%v", val)
	}
}

func (s *ReceiveStep) ToYAML() map[string]interface{} {
	if strings.TrimSpace(s.TargetURL) != "" {
		return map[string]interface{}{"receive": strings.TrimSpace(s.TargetURL)}
	}
	return nil
}

func (s *ReceiveStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "TargetURL", Label: "url", Value: s.TargetURL},
	}
}

func (s *ReceiveStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "TargetURL" {
		s.TargetURL = value
	}
	return nil
}

func (s *ReceiveStep) AddArrayItem(field string) {}
func (s *ReceiveStep) RemoveArrayItem(field string, index int) {}

type ReportStep struct {
	Targets []ActionTarget
}

func (s *ReportStep) Type() StepType { return StepReport }

func (s *ReportStep) FromYAML(stepMap map[string]interface{}) {
	if val, ok := stepMap["report"]; ok {
		if rArr, ok := val.([]interface{}); ok {
			for _, r := range rArr {
				if rMap, ok := r.(map[string]interface{}); ok {
					var at ActionTarget
					for k, v := range rMap {
						if k == "basic_auth" {
							if authMap, ok := v.(map[string]interface{}); ok {
								if u, ok := authMap["username"].(string); ok { at.Username = u }
								if p, ok := authMap["password"].(string); ok { at.Password = p }
							}
						} else if k == "method" {
							if m, ok := v.(string); ok { at.Method = m }
						} else if k == "headers" {
							if hMap, ok := v.(map[string]interface{}); ok {
								var hStrs []string
								for hk, hv := range hMap {
									hStrs = append(hStrs, fmt.Sprintf("%s: %v", hk, hv))
								}
								at.Headers = strings.Join(hStrs, ", ")
							}
						} else if k == "payload_template" {
							if p, ok := v.(string); ok { at.PayloadTemplate = p }
						} else {
							if str, ok := v.(string); ok {
								at.Type = k
								at.Target = str
							}
						}
					}
					s.Targets = append(s.Targets, at)
				}
			}
		}
	}
}

func (s *ReportStep) ToYAML() map[string]interface{} {
	var validTargets []map[string]interface{}
	for _, t := range s.Targets {
		cleanType := strings.TrimSpace(t.Type)
		cleanTarget := strings.TrimSpace(t.Target)
		if cleanType != "" && cleanTarget != "" {
			tMap := map[string]interface{}{cleanType: cleanTarget}
			
			if cleanType == "ntfy" {
				if t.Username != "" || t.Password != "" {
					tMap["basic_auth"] = map[string]interface{}{
						"username": t.Username,
						"password": t.Password,
					}
				}
			} else if cleanType == "http_request" {
				if t.Method != "" {
					tMap["method"] = t.Method
				}
				if t.Headers != "" {
					headersMap := make(map[string]interface{})
					for _, h := range strings.Split(t.Headers, ",") {
						parts := strings.SplitN(h, ":", 2)
						if len(parts) == 2 {
							headersMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
						}
					}
					if len(headersMap) > 0 {
						tMap["headers"] = headersMap
					}
				}
				if t.PayloadTemplate != "" {
					tMap["payload_template"] = t.PayloadTemplate
				}
			}
			
			validTargets = append(validTargets, tMap)
		}
	}
	
	if len(validTargets) > 0 {
		return map[string]interface{}{"report": validTargets}
	}
	return nil
}

func (s *ReportStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	if len(s.Targets) == 0 {
		s.Targets = []ActionTarget{{}}
	}
	for i, t := range s.Targets {
		typeLabel := "type"
		if i > 0 { typeLabel = "" }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportType", Label: typeLabel, Value: t.Type})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportTarget", Label: "target", Value: t.Target})
		
		if t.Type == "ntfy" {
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportUsername", Label: "username", Value: t.Username})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportPassword", Label: "password", Value: t.Password})
		} else if t.Type == "http_request" {
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportMethod", Label: "method", Value: t.Method})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportHeaders", Label: "headers", Value: t.Headers})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "ReportPayload", Label: "payload", Value: t.PayloadTemplate})
		}
		
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: i, Field: "ReportRemoveTarget", Label: "[ - Remove Target ]"})
		if i < len(s.Targets)-1 {
			rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
		}
	}
	rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, Field: "EditOnlySpacer"})
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportAddTarget", Label: "[ + Add Target ]"})
	return rows
}

func (s *ReportStep) UpdateField(field string, targetIndex int, value string) error {
	if targetIndex >= 0 && targetIndex < len(s.Targets) {
		if field == "ReportType" {
			s.Targets[targetIndex].Type = value
		} else if field == "ReportTarget" {
			s.Targets[targetIndex].Target = value
		} else if field == "ReportUsername" {
			s.Targets[targetIndex].Username = value
		} else if field == "ReportPassword" {
			s.Targets[targetIndex].Password = value
		} else if field == "ReportMethod" {
			s.Targets[targetIndex].Method = value
		} else if field == "ReportHeaders" {
			s.Targets[targetIndex].Headers = value
		} else if field == "ReportPayload" {
			s.Targets[targetIndex].PayloadTemplate = value
		}
	}
	return nil
}

func (s *ReportStep) AddArrayItem(field string) {
	if field == "ReportAddTarget" {
		s.Targets = append(s.Targets, ActionTarget{})
	}
}

func (s *ReportStep) RemoveArrayItem(field string, index int) {
	if (field == "ReportType" || field == "ReportTarget" || field == "ReportRemoveTarget") && index >= 0 && index < len(s.Targets) {
		s.Targets = append(s.Targets[:index], s.Targets[index+1:]...)
	}
}
