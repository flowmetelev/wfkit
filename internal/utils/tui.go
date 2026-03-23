package utils

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

type SummaryMetric struct {
	Label string
	Value string
	Tone  string
}

type ActionCard struct {
	Title       string
	Description string
	Command     string
}

type DashboardCard struct {
	Title   string
	Tone    string
	Metrics []SummaryMetric
	Lines   []string
}

type TimelineStep struct {
	Label   string
	Status  string
	Details string
}

var (
	uiBrandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#0F766E")).
			Padding(0, 1)
	uiVersionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#D1FAE5")).
			Padding(0, 1)
	uiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0"))
	uiMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8"))
	uiSectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#CFFAFE")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#155E75")).
			Padding(0, 1)
	uiLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Width(10)
	uiValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Bold(true)
	uiCommandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.Color("#1E293B")).
			Padding(0, 1)
	uiCardStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#1E293B")).
			Padding(1, 2).
			Width(28).
			Height(6)
	uiCardTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F8FAFC"))
	uiCardCommandStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#67E8F9"))
	uiSuccessBoxStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#15803D")).
				Padding(1, 2)
	uiErrorBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#B91C1C")).
			Padding(1, 2)
	uiTimelineLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E2E8F0")).
				Bold(true)
)

func PrintAppHeader(version, subtitle string) {
	parts := []string{uiBrandStyle.Render("wfkit")}
	if strings.TrimSpace(version) != "" {
		parts = append(parts, uiVersionStyle.Render("v"+version))
	}
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
	if strings.TrimSpace(subtitle) != "" {
		fmt.Println(uiMutedStyle.Render(subtitle))
	}
	fmt.Println()
}

func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

func PrintSection(title string) {
	fmt.Println(uiSectionStyle.Render(title))
}

func PrintKeyValue(label, value string) {
	fmt.Printf("%s %s\n", uiLabelStyle.Render(label+":"), uiValueStyle.Render(value))
}

func PrintStatus(status, title, message string) {
	badge := statusBadge(status)
	if strings.TrimSpace(message) == "" {
		fmt.Printf("%s %s\n", badge, uiValueStyle.Render(title))
		return
	}
	fmt.Printf("%s %s %s\n", badge, uiValueStyle.Render(title), uiMutedStyle.Render(message))
}

func PrintSummary(metrics ...SummaryMetric) {
	parts := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		if strings.TrimSpace(metric.Label) == "" || strings.TrimSpace(metric.Value) == "" {
			continue
		}
		tone := metric.Tone
		if tone == "" {
			tone = "info"
		}
		parts = append(parts, toneBadge(tone).Render(metric.Value)+" "+uiMutedStyle.Render(metric.Label))
	}
	if len(parts) == 0 {
		return
	}
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, parts...))
}

func PrintCommandHints(commands ...string) {
	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue
		}
		fmt.Printf("  %s\n", uiCommandStyle.Render(command))
	}
}

func PrintActionCards(cards ...ActionCard) {
	if len(cards) == 0 {
		return
	}

	rendered := make([]string, 0, len(cards))
	for _, card := range cards {
		body := []string{
			uiCardTitleStyle.Render(card.Title),
			uiMutedStyle.Render(card.Description),
		}
		if strings.TrimSpace(card.Command) != "" {
			body = append(body, "", uiCardCommandStyle.Render(card.Command))
		}
		rendered = append(rendered, uiCardStyle.Render(strings.Join(body, "\n")))
	}

	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, rendered...))
	fmt.Println()
}

func PrintUpdateBanner(currentVersion, latestVersion string) {
	PrintSection("Update Available")
	PrintStatus("WARN", "New version detected", "")
	if strings.TrimSpace(currentVersion) != "" {
		PrintKeyValue("Current", currentVersion)
	}
	if strings.TrimSpace(latestVersion) != "" {
		PrintKeyValue("Latest", latestVersion)
	}
	PrintCommandHints("npm install -g wfkit@latest")
	fmt.Println()
}

func PrintDashboardCards(cards ...DashboardCard) {
	if len(cards) == 0 {
		return
	}

	rendered := make([]string, 0, len(cards))
	for _, card := range cards {
		cardStyle := uiCardStyle.Copy().Height(8)
		switch strings.ToLower(strings.TrimSpace(card.Tone)) {
		case "success":
			cardStyle = cardStyle.BorderForeground(lipgloss.Color("#15803D"))
		case "warning":
			cardStyle = cardStyle.BorderForeground(lipgloss.Color("#B45309"))
		case "danger":
			cardStyle = cardStyle.BorderForeground(lipgloss.Color("#B91C1C"))
		default:
			cardStyle = cardStyle.BorderForeground(lipgloss.Color("#2563EB"))
		}

		body := []string{uiCardTitleStyle.Render(card.Title)}
		if len(card.Metrics) > 0 {
			parts := make([]string, 0, len(card.Metrics))
			for _, metric := range card.Metrics {
				if metric.Label == "" || metric.Value == "" {
					continue
				}
				tone := metric.Tone
				if tone == "" {
					tone = card.Tone
				}
				parts = append(parts, toneBadge(tone).Render(metric.Value)+" "+uiMutedStyle.Render(metric.Label))
			}
			if len(parts) > 0 {
				body = append(body, "", lipgloss.JoinHorizontal(lipgloss.Top, parts...))
			}
		}
		for _, line := range card.Lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			body = append(body, uiMutedStyle.Render(line))
		}
		rendered = append(rendered, cardStyle.Render(strings.Join(body, "\n")))
	}

	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top, rendered...))
	fmt.Println()
}

func PrintTimeline(title string, steps ...TimelineStep) {
	if title != "" {
		PrintSection(title)
	}
	for _, step := range steps {
		if strings.TrimSpace(step.Label) == "" {
			continue
		}
		badge := statusBadge(step.Status)
		if strings.TrimSpace(step.Details) == "" {
			fmt.Printf("%s %s\n", badge, uiTimelineLabelStyle.Render(step.Label))
			continue
		}
		fmt.Printf("%s %s %s\n", badge, uiTimelineLabelStyle.Render(step.Label), uiMutedStyle.Render(step.Details))
	}
	fmt.Println()
}

func PrintSuccessScreen(title, subtitle string, metrics []SummaryMetric, commands ...string) {
	lines := []string{uiTitleStyle.Render(title)}
	if strings.TrimSpace(subtitle) != "" {
		lines = append(lines, uiMutedStyle.Render(subtitle))
	}
	if len(metrics) > 0 {
		parts := make([]string, 0, len(metrics))
		for _, metric := range metrics {
			if metric.Label == "" || metric.Value == "" {
				continue
			}
			tone := metric.Tone
			if tone == "" {
				tone = "success"
			}
			parts = append(parts, toneBadge(tone).Render(metric.Value)+" "+uiMutedStyle.Render(metric.Label))
		}
		if len(parts) > 0 {
			lines = append(lines, "", lipgloss.JoinHorizontal(lipgloss.Top, parts...))
		}
	}
	if len(commands) > 0 {
		lines = append(lines, "", uiMutedStyle.Render("Next steps"))
		for _, command := range commands {
			if strings.TrimSpace(command) == "" {
				continue
			}
			lines = append(lines, "  "+uiCommandStyle.Render(command))
		}
	}

	fmt.Println(uiSuccessBoxStyle.Render(strings.Join(lines, "\n")))
	fmt.Println()
}

func PrintErrorScreen(title string, err error, hints ...string) {
	lines := []string{uiTitleStyle.Render(title)}
	if err != nil {
		wrapped := wrapErrorMessage(err.Error(), 88)
		if len(wrapped) > 0 {
			lines = append(lines, "")
			lines = append(lines, toneBadge("danger").Render("ERROR")+" "+uiMutedStyle.Render(wrapped[0]))
			for _, line := range wrapped[1:] {
				lines = append(lines, "      "+uiMutedStyle.Render(line))
			}
		}
	}
	if len(hints) > 0 {
		lines = append(lines, "", uiMutedStyle.Render("Next steps"))
		for _, hint := range hints {
			if strings.TrimSpace(hint) == "" {
				continue
			}
			wrapped := wrapErrorMessage(hint, 84)
			for i, line := range wrapped {
				prefix := "  "
				if i > 0 {
					prefix = "    "
				}
				lines = append(lines, prefix+uiMutedStyle.Render(line))
			}
		}
	}

	fmt.Fprintln(os.Stderr, uiErrorBoxStyle.Render(strings.Join(lines, "\n")))
	fmt.Fprintln(os.Stderr)
}

func wrapErrorMessage(message string, width int) []string {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	if width <= 0 {
		width = 88
	}

	var result []string
	for _, paragraph := range strings.Split(message, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		current := words[0]
		for _, word := range words[1:] {
			candidate := current + " " + word
			if utf8.RuneCountInString(candidate) <= width {
				current = candidate
				continue
			}
			result = append(result, current)
			current = word
		}
		result = append(result, splitLongWord(current, width)...)
	}

	return result
}

func splitLongWord(value string, width int) []string {
	if utf8.RuneCountInString(value) <= width {
		return []string{value}
	}

	runes := []rune(value)
	var lines []string
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		lines = append(lines, string(runes))
	}
	return lines
}

func RunTask(label string, fn func() error) error {
	frames := []string{"-", "\\", "|", "/"}
	done := make(chan struct{})
	var once sync.Once

	go func() {
		ticker := time.NewTicker(90 * time.Millisecond)
		defer ticker.Stop()
		index := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Printf("\r%s %s", toneBadge("info").Render(frames[index%len(frames)]), uiMutedStyle.Render(label))
				index++
			}
		}
	}()

	stop := func() {
		once.Do(func() {
			close(done)
			fmt.Print("\r\033[K")
		})
	}

	err := fn()
	stop()
	if err != nil {
		PrintStatus("FAIL", label, err.Error())
		return err
	}
	PrintStatus("OK", label, "")
	return nil
}

func statusBadge(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "PASS", "OK", "READY", "PUSHED":
		return toneBadge("success").Render(strings.ToUpper(strings.TrimSpace(status)))
	case "WARN", "SKIP", "MIGRATE", "UPDATE":
		return toneBadge("warning").Render(strings.ToUpper(strings.TrimSpace(status)))
	case "FAIL", "ERROR":
		return toneBadge("danger").Render(strings.ToUpper(strings.TrimSpace(status)))
	default:
		return toneBadge("info").Render(strings.ToUpper(strings.TrimSpace(status)))
	}
}

func toneBadge(tone string) lipgloss.Style {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F8FAFC")).
		Padding(0, 1)

	switch strings.ToLower(strings.TrimSpace(tone)) {
	case "success":
		return style.Background(lipgloss.Color("#15803D"))
	case "warning":
		return style.Background(lipgloss.Color("#B45309"))
	case "danger":
		return style.Background(lipgloss.Color("#B91C1C"))
	default:
		return style.Background(lipgloss.Color("#2563EB"))
	}
}
