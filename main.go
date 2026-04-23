package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Data model ────────────────────────────────────────────────────────────────

type Argument struct {
	Name        string
	Type        string
	Required    string
	Description string
}

type Skill struct {
	Name        string
	Description string
	Arguments   []Argument
	ExtraMeta   map[string]string
}

// ── Parser ────────────────────────────────────────────────────────────────────

func parseSkillMD(path string) (Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer f.Close()

	skill := Skill{ExtraMeta: map[string]string{}}
	scanner := bufio.NewScanner(f)

	// --- frontmatter ---
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	inFront := false
	frontDone := false
	fmCount := 0
	var bodyLines []string

	for _, line := range lines {
		if !frontDone {
			if strings.TrimSpace(line) == "---" {
				fmCount++
				if fmCount == 1 {
					inFront = true
					continue
				} else if fmCount == 2 {
					frontDone = true
					inFront = false
					continue
				}
			}
			if inFront {
				// key: value (simple, no nested YAML)
				idx := strings.Index(line, ":")
				if idx > 0 {
					key := strings.TrimSpace(line[:idx])
					val := strings.TrimSpace(line[idx+1:])
					val = strings.Trim(val, `"'`)
					switch key {
					case "name":
						skill.Name = val
					case "description":
						skill.Description = val
					default:
						skill.ExtraMeta[key] = val
					}
				}
				continue
			}
		}
		bodyLines = append(bodyLines, line)
	}

	if skill.Name == "" {
		skill.Name = filepath.Base(filepath.Dir(path))
	}

	// Multiline description: join continuation lines (indented or just long yaml value)
	// description may span multiple lines in frontmatter as a quoted string.
	// Re-scan frontmatter for multiline description.
	skill.Description = parseMultilineDesc(lines)

	skill.Arguments = parseArguments(bodyLines)
	return skill, nil
}

// parseMultilineDesc handles description values that may be long quoted strings.
func parseMultilineDesc(lines []string) string {
	inFront := false
	fmCount := 0
	inDesc := false
	var descParts []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "---" {
			fmCount++
			if fmCount == 1 {
				inFront = true
				continue
			} else if fmCount == 2 {
				break
			}
		}
		if !inFront {
			continue
		}
		idx := strings.Index(line, ":")
		if idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if key == "description" {
				inDesc = true
				val = strings.TrimLeft(val, `"'`)
				// if closing quote on same line
				if strings.HasSuffix(val, `"`) || strings.HasSuffix(val, `'`) {
					val = strings.TrimRight(val, `"'`)
					return val
				}
				descParts = append(descParts, val)
				continue
			} else if inDesc {
				// new key — description ended
				inDesc = false
			}
		} else if inDesc {
			trimmed := strings.TrimSpace(line)
			trimmed = strings.TrimRight(trimmed, `"'`)
			descParts = append(descParts, trimmed)
		}
	}
	if len(descParts) > 0 {
		return strings.Join(descParts, " ")
	}
	return ""
}

func parseArguments(bodyLines []string) []Argument {
	var args []Argument
	inSection := false

	for i, line := range bodyLines {
		lower := strings.ToLower(strings.TrimSpace(line))
		// Detect section headers
		if strings.HasPrefix(lower, "#") &&
			(strings.Contains(lower, "argument") ||
				strings.Contains(lower, "parameter") ||
				strings.Contains(lower, "option") ||
				strings.Contains(lower, "input")) {
			inSection = true
			continue
		}
		// End section at next heading
		if inSection && strings.HasPrefix(strings.TrimSpace(line), "#") {
			break
		}
		if !inSection {
			continue
		}

		// Markdown table row: |name|type|req|desc|
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			cells := strings.Split(line, "|")
			if len(cells) < 3 {
				continue
			}
			// strip leading/trailing empty from split
			cells = cells[1 : len(cells)-1]
			for j := range cells {
				cells[j] = strings.TrimSpace(cells[j])
			}
			// skip separator or header
			if len(cells) == 0 || strings.Repeat("-", len(cells[0])) == cells[0] {
				continue
			}
			if strings.ToLower(cells[0]) == "name" || strings.ToLower(cells[0]) == "parameter" {
				continue
			}
			arg := Argument{Name: cells[0]}
			if len(cells) > 1 {
				arg.Type = cells[1]
			}
			if len(cells) > 2 {
				arg.Required = cells[2]
			}
			if len(cells) > 3 {
				arg.Description = cells[3]
			}
			args = append(args, arg)
			continue
		}

		// Bullet: - `name` (type, required): desc
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			rest := trimmed[2:]
			// extract name (may be backtick-quoted)
			name := ""
			if strings.HasPrefix(rest, "`") {
				end := strings.Index(rest[1:], "`")
				if end >= 0 {
					name = rest[1 : end+1]
					rest = strings.TrimSpace(rest[end+2:])
				}
			} else {
				sp := strings.IndexAny(rest, " \t(:")
				if sp > 0 {
					name = rest[:sp]
					rest = strings.TrimSpace(rest[sp:])
				} else {
					name = rest
					rest = ""
				}
			}
			typeStr, reqStr := "", ""
			if strings.HasPrefix(rest, "(") {
				end := strings.Index(rest, ")")
				if end > 0 {
					parts := strings.SplitN(rest[1:end], ",", 2)
					typeStr = strings.TrimSpace(parts[0])
					if len(parts) > 1 {
						reqStr = strings.TrimSpace(parts[1])
					}
					rest = strings.TrimSpace(rest[end+1:])
				}
			}
			desc := strings.TrimLeft(rest, ": ")
			if name != "" {
				args = append(args, Argument{
					Name:        name,
					Type:        typeStr,
					Required:    reqStr,
					Description: desc,
				})
			}
		}
		_ = i
	}
	return args
}

func loadSkills(dir string) ([]Skill, error) {
	var skills []Skill
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || strings.ToUpper(info.Name()) != "SKILL.MD" {
			return nil
		}
		s, err := parseSkillMD(path)
		if err != nil {
			return nil
		}
		skills = append(skills, s)
		return nil
	})
	return skills, err
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#888888", Dark: "#666666"}
	highlight = lipgloss.AdaptiveColor{Light: "#00AACC", Dark: "#00CCFF"}
	special   = lipgloss.AdaptiveColor{Light: "#CC6600", Dark: "#FFAA44"}
	white     = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#EEEEEE"}

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00CCFF")).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00CCFF")).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFCC00"))

	normalItemStyle = lipgloss.NewStyle().
			Foreground(white)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(special).
			Underline(true)

	metaStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(subtle)
)

// ── Model ─────────────────────────────────────────────────────────────────────

type model struct {
	skills       []Skill
	selected     int
	listScroll   int
	detailScroll int
	width        int
	height       int
}

func initialModel(skills []Skill) model {
	return model{skills: skills}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m.detailScroll = 0
				m = m.syncListScroll()
			}
		case "down", "j":
			if m.selected < len(m.skills)-1 {
				m.selected++
				m.detailScroll = 0
				m = m.syncListScroll()
			}
		case "pgup", "ctrl+u":
			m.detailScroll -= m.innerHeight()
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
		case "pgdown", "ctrl+d":
			m.detailScroll += m.innerHeight()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.syncListScroll()
	}
	return m, nil
}

// syncListScroll keeps listScroll in sync with selected — called only from Update.
func (m model) syncListScroll() model {
	h := m.innerHeight()
	if h < 1 {
		return m
	}
	if m.selected < m.listScroll {
		m.listScroll = m.selected
	} else if m.selected >= m.listScroll+h {
		m.listScroll = m.selected - h + 1
	}
	return m
}

func (m model) innerHeight() int {
	h := m.height - 4 // header(1) + footer(1) + border top(1) + border bottom(1)
	if h < 1 {
		return 1
	}
	return h
}

func (m model) detailHeight() int {
	return m.innerHeight()
}

func (m model) leftWidth() int {
	w := m.width / 3
	if w < 22 {
		w = 22
	}
	if w > 36 {
		w = 36
	}
	return w
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	lw := m.leftWidth()
	rw := m.width - lw

	// ── header ──
	header := headerStyle.Render("  skill_browser  ↑/↓ j/k select  PgUp/PgDn scroll  q quit  ")
	header = padRight(header, m.width)

	// ── left pane ──
	innerH := m.innerHeight()

	var leftItems []string
	for i := m.listScroll; i < m.listScroll+innerH && i < len(m.skills); i++ {
		name := " " + m.skills[i].Name + " "
		name = truncate(name, lw-4)
		name = padRight(name, lw-4)
		if i == m.selected {
			leftItems = append(leftItems, selectedStyle.Render(name))
		} else {
			leftItems = append(leftItems, normalItemStyle.Render(name))
		}
	}
	// pad to fill inner height
	for len(leftItems) < innerH {
		leftItems = append(leftItems, strings.Repeat(" ", lw-4))
	}

	leftContent := strings.Join(leftItems, "\n")
	leftPane := borderStyle.
		Width(lw - 2).
		Height(innerH).
		Render(leftContent)

	// ── right pane ──
	var detailLines []string
	if len(m.skills) > 0 {
		detailLines = buildDetail(m.skills[m.selected], rw-4)
	}

	// clamp scroll
	maxScroll := len(detailLines) - innerH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}

	visible := detailLines
	if m.detailScroll < len(detailLines) {
		visible = detailLines[m.detailScroll:]
	} else {
		visible = nil
	}
	if len(visible) > innerH {
		visible = visible[:innerH]
	}
	for len(visible) < innerH {
		visible = append(visible, "")
	}

	rightContent := strings.Join(visible, "\n")
	rightPane := borderStyle.
		Width(rw - 2).
		Height(innerH).
		Render(rightContent)

	// ── join panes ──
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// ── footer ──
	footerLeft := fmt.Sprintf("  %d/%d  ", m.selected+1, len(m.skills))
	footerRight := "  ↑/↓ j/k: select skill   PgUp/PgDn: scroll detail   q: quit  "
	gap := m.width - len([]rune(footerLeft)) - len([]rune(footerRight))
	if gap < 0 {
		gap = 0
	}
	footer := footerStyle.Render(footerLeft + strings.Repeat(" ", gap) + footerRight)

	return header + "\n" + panes + "\n" + footer
}

// buildDetail returns the rendered lines for the right pane.
func buildDetail(s Skill, width int) []string {
	var lines []string

	add := func(line string) { lines = append(lines, line) }
	blank := func() { add("") }

	// Title
	add(titleStyle.Render("  " + s.Name))
	blank()

	// Description
	add(labelStyle.Render("  Description"))
	for _, l := range wordWrap(s.Description, width-4) {
		add("  " + normalItemStyle.Render(l))
	}
	blank()

	// Extra metadata
	if len(s.ExtraMeta) > 0 {
		add(labelStyle.Render("  Metadata"))
		for k, v := range s.ExtraMeta {
			for _, l := range wordWrap(k+": "+v, width-6) {
				add("    " + metaStyle.Render(l))
			}
		}
		blank()
	}

	// Arguments
	add(labelStyle.Render("  Arguments"))
	if len(s.Arguments) == 0 {
		add(dimStyle.Render("    (none defined in SKILL.md)"))
	} else {
		for _, arg := range s.Arguments {
			header := "    • " + arg.Name
			if arg.Type != "" {
				header += "  " + metaStyle.Render("("+arg.Type+")")
			}
			if arg.Required != "" {
				header += "  " + metaStyle.Render("["+arg.Required+"]")
			}
			add(header)
			if arg.Description != "" {
				for _, l := range wordWrap(arg.Description, width-10) {
					add("        " + dimStyle.Render(l))
				}
			}
		}
	}
	blank()

	return lines
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func wordWrap(text string, width int) []string {
	if width <= 0 {
		width = 40
	}
	var result []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		line := ""
		for _, w := range words {
			if line == "" {
				line = w
			} else if len(line)+1+len(w) <= width {
				line += " " + w
			} else {
				result = append(result, line)
				line = w
			}
		}
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func padRight(s string, n int) string {
	runes := []rune(s)
	if len(runes) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(runes))
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	skills, err := loadSkills(dir)
	if err != nil || len(skills) == 0 {
		fmt.Fprintf(os.Stderr, "No SKILL.md files found in: %s\n", dir)
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(skills), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

