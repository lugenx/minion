package character

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	lipgloss "github.com/charmbracelet/lipgloss"
)

type Stage int

const (
	Baby Stage = iota
	Child
	Adult
)

const MaxArtLines = 10

type Mood int

const (
	Sleeping Mood = iota
	Blinking
	Awake
)

type EyeCount int

const (
	One EyeCount = iota
	Two
)

type HairStyle string

const (
	StdOne HairStyle = "standard_one"
	StdTwo HairStyle = "standard_two"
	SpkOne HairStyle = "spiky_one"
	SpkTwo HairStyle = "spiky_two"
	CrlOne HairStyle = "curly_one"
	CrlTwo HairStyle = "curly_two"
	BnnOne HairStyle = "beanie_one"
	BnnTwo HairStyle = "beanie_two"
)

type hairInfo struct {
	Line string
	Eyes EyeCount
}

var hairDefs = map[HairStyle]hairInfo{
	StdOne: {"___", One},  StdTwo: {"___", Two},
	SpkOne: {`\|/`, One},  SpkTwo: {`\|/`, Two},
	CrlOne: {" @ ", One},  CrlTwo: {" @ ", Two},
	BnnOne: {".~.", One},  BnnTwo: {".~.", Two},
}

type Data struct {
	TotalRuns    int
	TotalMatches int
	LastResults  int
	LastErrors   int
	HairStyle    HairStyle
	CreatedAt    time.Time
}

var allHairStyles = []HairStyle{StdOne, StdTwo, SpkOne, SpkTwo, CrlOne, CrlTwo, BnnOne, BnnTwo}

func AllHairStyles() []HairStyle {
	return allHairStyles
}

func RandomHairStyle() HairStyle {
	return allHairStyles[rand.Intn(len(allHairStyles))]
}

func Enabled() bool {
	return strings.ToLower(os.Getenv("MINION_CHARACTER")) != "false"
}

func babyFaceLine(eyes EyeCount, mood Mood) string {
	switch mood {
	case Sleeping, Blinking:
		if eyes == One {
			return "|---|"
		}
		return "|- -|"
	default:
		if eyes == One {
			return "|-o-|"
		}
		return "|o-o|"
	}
}

func adultFaceLine(eyes EyeCount, mood Mood) string {
	switch mood {
	case Sleeping, Blinking:
		if eyes == One {
			return "|-(-)-|"
		}
		return "|-----|"
	default:
		if eyes == One {
			return "|-(o)-|"
		}
		return "|-o-o-|"
	}
}

func babyArt(h hairInfo, mood Mood) []string {
	return []string{h.Line, babyFaceLine(h.Eyes, mood), "|===|", ` " " `}
}

func childArt(h hairInfo, mood Mood) []string {
	face := babyFaceLine(h.Eyes, mood)
	return []string{"  " + h.Line + "  ", " " + face + " ", " | - | ", "(|===|)", ` "   " `}
}

func adultArt(h hairInfo, mood Mood) []string {
	face := adultFaceLine(h.Eyes, mood)
	return []string{
		"    " + h.Line + "   ",
		"   /   \\  ",
		"  " + face + " ",
		"  |  -  | ",
		" /| === |\\",
		"  |=====| ",
		"  |_____| ",
		"   || ||  ",
		`   "" ""  `,
	}
}

func Art(hs HairStyle, stage Stage, mood Mood) []string {
	h, ok := hairDefs[hs]
	if !ok {
		return nil
	}
	switch stage {
	case Child:
		return childArt(h, mood)
	case Adult:
		return adultArt(h, mood)
	default:
		return babyArt(h, mood)
	}
}

func GetStage(birthTime time.Time) Stage {
	if birthTime.IsZero() {
		return Baby
	}
	age := time.Since(birthTime)
	switch {
	case age >= 30*24*time.Hour:
		return Adult
	case age >= 7*24*time.Hour:
		return Child
	default:
		return Baby
	}
}

func GetMood(disabled bool, isStarted bool) Mood {
	if disabled || !isStarted {
		return Sleeping
	}
	return Awake
}

func FormatAge(t time.Time) string {
	age := time.Since(t)
	switch {
	case age < 24*time.Hour:
		return "Newborn"
	case age < 7*24*time.Hour:
		d := int(age.Hours() / 24)
		if d == 1 {
			return "1 day old"
		}
		return fmt.Sprintf("%d days old", d)
	case age < 30*24*time.Hour:
		w := int(age.Hours() / (24 * 7))
		if w == 1 {
			return "1 week old"
		}
		return fmt.Sprintf("%d weeks old", w)
	case age < 365*24*time.Hour:
		m := int(age.Hours() / (24 * 30))
		if m == 1 {
			return "1 month old"
		}
		return fmt.Sprintf("%d months old", m)
	default:
		y := int(age.Hours() / (24 * 365))
		if y == 1 {
			return "1 year old"
		}
		return fmt.Sprintf("%d years old", y)
	}
}

var (
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	blueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4169E1"))
)

func colorPantsLine(line string) string {
	var b strings.Builder
	for _, r := range line {
		switch {
		case r == ' ':
			b.WriteRune(' ')
		case r == '=' || r == '_':
			b.WriteString(blueStyle.Render(string(r)))
		default:
			b.WriteString(yellowStyle.Render(string(r)))
		}
	}
	return b.String()
}

func ColorLine(stage Stage, idx int, line string) string {
	if idx == 0 {
		var b strings.Builder
		for _, r := range line {
			if r != ' ' {
				b.WriteString(yellowStyle.Render(string(r)))
			} else {
				b.WriteRune(' ')
			}
		}
		return b.String()
	}

	switch stage {
	case Adult:
		switch {
		case idx <= 3:
			return yellowStyle.Render(line)
		case idx <= 6:
			return colorPantsLine(line)
		default:
			return blueStyle.Render(line)
		}
	case Child:
		switch {
		case idx <= 2:
			return yellowStyle.Render(line)
		case idx == 3:
			return colorPantsLine(line)
		default:
			return blueStyle.Render(line)
		}
	default: // Baby
		switch {
		case idx == 1:
			return yellowStyle.Render(line)
		case idx == 2:
			return colorPantsLine(line)
		default:
			return blueStyle.Render(line)
		}
	}
}
