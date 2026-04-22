package schedule

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

// ParseToCron converts custom schedule strings to a standard cron expression.
func ParseToCron(schedule string) (string, error) {
	schedule = strings.ToLower(strings.TrimSpace(schedule))

	// Case 1: Interval Alternative (e.g., "every 30m", "every 12h")
	if strings.HasPrefix(schedule, "every ") {
		duration := strings.TrimSpace(strings.TrimPrefix(schedule, "every"))
		return "@every " + duration, nil
	}

	// Case 2: Custom @ Syntax
	if strings.Contains(schedule, "@") {
		parts := strings.Split(schedule, "@")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid schedule format, expected 'days @ time', got: %s", schedule)
		}

		daysPart := strings.TrimSpace(parts[0])
		timePart := strings.TrimSpace(parts[1])

		// Parse Time
		timeParts := strings.Split(timePart, ":")
		if len(timeParts) != 2 {
			return "", fmt.Errorf("invalid time format, expected HH:MM, got: %s", timePart)
		}
		hour := timeParts[0]
		minute := timeParts[1]

		// Parse Days
		if daysPart == "daily" {
			return fmt.Sprintf("%s %s * * *", minute, hour), nil
		}
		if daysPart == "weekdays" {
			return fmt.Sprintf("%s %s * * 1,2,3,4,5", minute, hour), nil
		}
		if daysPart == "weekends" {
			return fmt.Sprintf("%s %s * * 0,6", minute, hour), nil
		}

		dayMap := map[string]string{
			"sun": "0",
			"mon": "1",
			"tue": "2",
			"wed": "3",
			"thu": "4",
			"fri": "5",
			"sat": "6",
		}

		rawDays := strings.Split(daysPart, ",")
		var cronDays []string
		for _, rd := range rawDays {
			d := strings.TrimSpace(rd)
			if val, ok := dayMap[d]; ok {
				cronDays = append(cronDays, val)
			} else {
				return "", fmt.Errorf("invalid day abbreviation: %s", d)
			}
		}

		if len(cronDays) == 0 {
			return "", fmt.Errorf("no valid days provided")
		}

		return fmt.Sprintf("%s %s * * %s", minute, hour, strings.Join(cronDays, ",")), nil
	}

	// Case 3: Fallback to Raw Cron Validation
	// We use the same parser setup the rest of the app uses to validate it
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(schedule); err != nil {
		return "", fmt.Errorf("unrecognized format or invalid cron string: %v", err)
	}

	return schedule, nil
}