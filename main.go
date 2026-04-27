package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
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
	subtle    = compat.AdaptiveColor{Light: lipgloss.Color("#888888"), Dark: lipgloss.Color("#666666")}
	highlight = compat.AdaptiveColor{Light: lipgloss.Color("#00AACC"), Dark: lipgloss.Color("#00CCFF")}
	special   = compat.AdaptiveColor{Light: lipgloss.Color("#CC6600"), Dark: lipgloss.Color("#FFAA44")}
	white     = compat.AdaptiveColor{Light: lipgloss.Color("#000000"), Dark: lipgloss.Color("#EEEEEE")}

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

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFAA44"))

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
	skills     []Skill
	selected   int
	listScroll int
	width      int
	height     int

	// vi-like search state
	searchMode    bool   // true while typing query (after / or ?)
	searchInput   string // buffer being typed in search mode
	searchQuery   string // last submitted query (used by n/N)
	searchDir     int    // 1 forward, -1 backward — direction of last search
	searchMessage string // transient status (e.g., "Pattern not found")
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
		if m.searchMode {
			return m.updateSearchMode(msg), nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.searchMode = true
			m.searchDir = 1
			m.searchInput = ""
			m.searchMessage = ""
			return m, nil
		case "?":
			m.searchMode = true
			m.searchDir = -1
			m.searchInput = ""
			m.searchMessage = ""
			return m, nil
		case "n":
			if m.searchQuery != "" {
				m = m.findMatch(m.selected, m.searchDir, false)
			}
		case "N":
			if m.searchQuery != "" {
				dir := -m.searchDir
				if dir == 0 {
					dir = -1
				}
				m = m.findMatch(m.selected, dir, false)
			}
		case "up", "k":
			if m.selected > 0 {
				m.selected--
				m = m.syncListScroll()
			}
		case "down", "j":
			if m.selected < len(m.skills)-1 {
				m.selected++
				m = m.syncListScroll()
			}
		case "pgup", "ctrl+u":
			step := m.innerHeight()
			if step < 1 {
				step = 1
			}
			m.selected -= step
			if m.selected < 0 {
				m.selected = 0
			}
			m = m.syncListScroll()
		case "pgdown", "ctrl+d":
			step := m.innerHeight()
			if step < 1 {
				step = 1
			}
			m.selected += step
			if m.selected > len(m.skills)-1 {
				m.selected = len(m.skills) - 1
			}
			m = m.syncListScroll()
		case "home", "g":
			m.selected = 0
			m = m.syncListScroll()
		case "end", "G":
			m.selected = len(m.skills) - 1
			m = m.syncListScroll()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m = m.syncListScroll()
	}
	return m, nil
}

func (m model) updateSearchMode(msg tea.KeyMsg) model {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.searchMode = false
		m.searchInput = ""
		m.searchMessage = ""
		return m
	case "enter":
		m.searchMode = false
		m.searchQuery = m.searchInput
		m.searchInput = ""
		if m.searchQuery != "" {
			m = m.findMatch(m.selected, m.searchDir, true)
		}
		return m
	case "backspace", "ctrl+h":
		runes := []rune(m.searchInput)
		if len(runes) > 0 {
			m.searchInput = string(runes[:len(runes)-1])
		}
		return m
	}
	if text := msg.Key().Text; text != "" {
		m.searchInput += text
	}
	return m
}

// skillMatches reports whether the skill's title or description contains the
// (case-insensitive) query. Empty query never matches.
func skillMatches(s Skill, lowerQuery string) bool {
	if lowerQuery == "" {
		return false
	}
	if strings.Contains(strings.ToLower(s.Name), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Description), lowerQuery) {
		return true
	}
	return false
}

// findMatch moves the selection to the next item that matches searchQuery
// (against title or description), searching in the given direction. Wraps
// around. If includeStart is true, the starting index itself is considered.
func (m model) findMatch(start, dir int, includeStart bool) model {
	if m.searchQuery == "" || len(m.skills) == 0 {
		return m
	}
	q := strings.ToLower(m.searchQuery)
	n := len(m.skills)
	begin := start
	if !includeStart {
		begin = start + dir
	}
	for i := 0; i < n; i++ {
		idx := ((begin+dir*i)%n + n) % n
		if skillMatches(m.skills[idx], q) {
			m.selected = idx
			m.searchMessage = ""
			return m.syncListScroll()
		}
	}
	m.searchMessage = "Pattern not found: " + m.searchQuery
	return m
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
	h := m.height - 4 // footer(2) + border top(1) + border bottom(1)
	if h < 1 {
		return 1
	}
	return h
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

func (m model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("Loading...")
	}

	lw := m.leftWidth()
	rw := m.width - lw

	// ── left pane: fixed height, list scrolls inside ──
	innerH := m.innerHeight()

	var leftItems []string
	matchQuery := strings.ToLower(m.searchQuery)
	for i := m.listScroll; i < m.listScroll+innerH && i < len(m.skills); i++ {
		name := " " + m.skills[i].Name + " "
		name = truncate(name, lw-4)
		name = padRight(name, lw-4)
		switch {
		case i == m.selected:
			leftItems = append(leftItems, selectedStyle.Render(name))
		case skillMatches(m.skills[i], matchQuery):
			leftItems = append(leftItems, matchStyle.Render(name))
		default:
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

	// ── right pane: render full detail content, height adapts ──
	var detailLines []string
	if len(m.skills) > 0 {
		detailLines = buildDetail(m.skills[m.selected], rw-4, matchQuery)
	}
	rightContent := strings.Join(detailLines, "\n")
	rightPaneStyle := borderStyle.Width(rw - 2)
	if len(detailLines) < innerH {
		rightPaneStyle = rightPaneStyle.Height(innerH)
	}
	rightPane := rightPaneStyle.Render(rightContent)

	// ── join panes (top-aligned so left list stays anchored) ──
	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// ── footer (also acts as vi-style command line) ──
	footer := m.renderFooter()

	v := tea.NewView(panes + "\n" + footer)
	v.AltScreen = true
	return v
}

func (m model) renderFooter() string {
	statusLine := m.renderStatusLine()
	hintsLine := m.renderHintsLine()
	return statusLine + "\n" + hintsLine
}

// renderStatusLine builds the top line of the footer: vi-style command line
// when typing a search, otherwise title + counter + last query/message.
func (m model) renderStatusLine() string {
	if m.searchMode {
		prefix := "/"
		if m.searchDir == -1 {
			prefix = "?"
		}
		line := "  " + prefix + m.searchInput + "█"
		return footerStyle.Render(padRight(line, m.width))
	}

	line := fmt.Sprintf("  skill_browser   %d/%d  ", m.selected+1, len(m.skills))
	if m.searchMessage != "" {
		line += " " + m.searchMessage + " "
	} else if m.searchQuery != "" {
		prefix := "/"
		if m.searchDir == -1 {
			prefix = "?"
		}
		line += " " + prefix + m.searchQuery + "  "
	}
	return footerStyle.Render(padRight(line, m.width))
}

// renderHintsLine builds the bottom line of the footer with key hints.
// Picks a layout that fits the available width; falls back to a compact form.
func (m model) renderHintsLine() string {
	full := "  j/k: select   PgUp/PgDn: page   g/G: top/bottom   /: search   n/N: next/prev   q: quit  "
	compact := "  j/k select   /: search   n/N next   q quit  "
	minimal := "  j/k  /search  q quit  "

	for _, hint := range []string{full, compact, minimal} {
		if len([]rune(hint)) <= m.width {
			return footerStyle.Render(padRight(hint, m.width))
		}
	}
	cut := min(len(minimal), m.width)
	return footerStyle.Render(padRight(minimal[:cut], m.width))
}

// buildDetail returns the rendered lines for the right pane.
// lowerQuery (already lower-cased) highlights matching substrings.
func buildDetail(s Skill, width int, lowerQuery string) []string {
	var lines []string

	add := func(line string) { lines = append(lines, line) }
	blank := func() { add("") }
	plainStyle := lipgloss.NewStyle()

	// Title
	add("  " + highlightMatches(s.Name, lowerQuery, titleStyle, matchStyle))
	blank()

	// Description
	add(labelStyle.Render("  Description"))
	for _, l := range wordWrap(s.Description, width-4) {
		add("  " + highlightMatches(l, lowerQuery, normalItemStyle, matchStyle))
	}
	blank()

	// Extra metadata
	if len(s.ExtraMeta) > 0 {
		add(labelStyle.Render("  Metadata"))
		for k, v := range s.ExtraMeta {
			for _, l := range wordWrap(k+": "+v, width-6) {
				add("    " + highlightMatches(l, lowerQuery, metaStyle, matchStyle))
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
			header := "    • " + highlightMatches(arg.Name, lowerQuery, plainStyle, matchStyle)
			if arg.Type != "" {
				header += "  " + metaStyle.Render("("+arg.Type+")")
			}
			if arg.Required != "" {
				header += "  " + metaStyle.Render("["+arg.Required+"]")
			}
			add(header)
			if arg.Description != "" {
				for _, l := range wordWrap(arg.Description, width-10) {
					add("        " + highlightMatches(l, lowerQuery, dimStyle, matchStyle))
				}
			}
		}
	}
	blank()

	return lines
}

// highlightMatches renders text with baseStyle, except occurrences of
// lowerQuery (case-insensitive substring match) which use hlStyle.
// Empty lowerQuery returns baseStyle.Render(text). Assumes byte length is
// preserved by ToLower (true for ASCII and CJK — fine for our inputs).
func highlightMatches(text, lowerQuery string, baseStyle, hlStyle lipgloss.Style) string {
	if lowerQuery == "" || text == "" {
		return baseStyle.Render(text)
	}
	lower := strings.ToLower(text)
	if len(lower) != len(text) {
		// ToLower changed byte length; fall back to plain styling to avoid
		// slicing into the middle of a rune.
		return baseStyle.Render(text)
	}
	qLen := len(lowerQuery)
	var b strings.Builder
	i := 0
	for {
		idx := strings.Index(lower[i:], lowerQuery)
		if idx < 0 {
			b.WriteString(baseStyle.Render(text[i:]))
			break
		}
		abs := i + idx
		if abs > i {
			b.WriteString(baseStyle.Render(text[i:abs]))
		}
		end := abs + qLen
		b.WriteString(hlStyle.Render(text[abs:end]))
		i = end
		if i >= len(text) {
			break
		}
	}
	return b.String()
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

	p := tea.NewProgram(initialModel(skills))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
