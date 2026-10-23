package ui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	"github.com/digitalis-io/cloudflare-dns-tui/internal/cloudflare"
)

// zonesLoadedMsg carries the result of a zones fetch.
type zonesLoadedMsg struct {
	zones []cloudflare.Zone
	err   error
}

// recordsLoadedMsg carries the result of a records fetch for a given sequence.
type recordsLoadedMsg struct {
	seq     int
	records []cloudflare.DNSRecord
	err     error
}

// spinnerTickMsg drives the spinner animation.
type spinnerTickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Model is the bubbletea TUI model.
type Model struct {
	client *cloudflare.Client
	width  int
	height int

	zones      []cloudflare.Zone
	zoneIdx    int
	records    []cloudflare.DNSRecord
	cursor     int
	offset     int
	loadSeq    int
	loading    bool
	err        string
	filterMode bool
	filter     string

	spinnerFrame int
}

// New creates a ready-to-run Model.
func New(client *cloudflare.Client) Model {
	return Model{client: client, width: 120, height: 30}
}

// Init starts the first data load and spinner.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadZones(m.client), spinnerTick())
}

// Update handles all incoming messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:cyclop
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinnerTickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTick()

	case zonesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.zones = msg.zones
		m.err = ""
		if len(m.zones) > 0 {
			return m, m.fetchRecords(0)
		}
		return m, nil

	case recordsLoadedMsg:
		if msg.seq != m.loadSeq {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.records = msg.records
		m.err = ""
		m.cursor = 0
		m.offset = 0
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.InterruptMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	switch msg.Code {
	case tea.KeyEsc, 'q':
		return m, tea.Quit
	case '\x03': // ctrl+c in raw mode
		return m, tea.Quit

	case tea.KeyUp, 'k':
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset--
			}
		}

	case tea.KeyDown, 'j':
		filtered := m.filtered()
		if m.cursor < len(filtered)-1 {
			m.cursor++
			if m.cursor >= m.offset+m.visibleRows() {
				m.offset++
			}
		}

	case tea.KeyLeft, 'p':
		if len(m.zones) > 1 && !m.loading {
			idx := (m.zoneIdx - 1 + len(m.zones)) % len(m.zones)
			return m, m.fetchRecords(idx)
		}

	case tea.KeyRight, 'n':
		if len(m.zones) > 1 && !m.loading {
			idx := (m.zoneIdx + 1) % len(m.zones)
			return m, m.fetchRecords(idx)
		}

	case 'r':
		if !m.loading {
			if len(m.zones) > 0 {
				return m, m.fetchRecords(m.zoneIdx)
			}
			m.loading = true
			return m, loadZones(m.client)
		}

	case '/':
		m.filterMode = true
		return m, nil
	}
	return m, nil
}

func (m Model) handleFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEsc:
		m.filterMode = false
		m.filter = ""
		m.cursor = 0
		m.offset = 0
	case tea.KeyEnter:
		m.filterMode = false
		m.cursor = 0
		m.offset = 0
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			_, size := utf8.DecodeLastRuneInString(m.filter)
			m.filter = m.filter[:len(m.filter)-size]
			m.cursor = 0
			m.offset = 0
		}
	default:
		if msg.Text != "" {
			m.filter += msg.Text
			m.cursor = 0
			m.offset = 0
		}
	}
	return m, nil
}

// View renders the entire TUI.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	var b strings.Builder

	b.WriteString(m.renderTitleBar())
	b.WriteString("\n")
	b.WriteString(m.renderZoneBar())
	b.WriteString("\n")
	b.WriteString(m.renderFilterBar())
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString(styleError.Render("✗ " + m.err))
		b.WriteString("\n")
	}

	b.WriteString(m.renderTable())
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m Model) renderTitleBar() string {
	title := styleTitleBar.Render("  Cloudflare DNS")
	spinner := ""
	if m.loading {
		spinner = "  " + styleSpinner.Render(spinnerFrames[m.spinnerFrame])
	}
	return title + spinner
}

func (m Model) renderZoneBar() string {
	if len(m.zones) == 0 {
		return styleStatus.Render("No zones loaded")
	}
	var parts []string
	for i, z := range m.zones {
		if i == m.zoneIdx {
			parts = append(parts, styleZoneTabActive.Render(z.Name))
		} else {
			parts = append(parts, styleZoneTab.Render(z.Name))
		}
	}
	hint := styleStatus.Render("  ←/→ switch zone")
	return strings.Join(parts, "") + hint
}

func (m Model) renderFilterBar() string {
	label := "Filter: "
	var content string
	if m.filterMode {
		content = styleFilter.Render(label + m.filter + "█")
	} else if m.filter != "" {
		content = styleFilterInactive.Render(label + m.filter + "  [esc clear]")
	} else {
		content = styleFilterInactive.Render(label + "  [/ to search]")
	}
	return content
}

func (m Model) renderTable() string {
	filtered := m.filtered()
	rows := m.visibleRows()
	if rows <= 0 {
		return ""
	}

	colType := 8
	colTTL := 7
	colProxy := 12
	remaining := m.width - colType - colTTL - colProxy - 8
	colName := remaining * 35 / 100
	colContent := remaining - colName

	header := m.renderRow("TYPE", "NAME", "CONTENT", "TTL", "PROXY",
		colType, colName, colContent, colTTL, colProxy, true, false)
	header = styleHeader.Render(header)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")

	end := m.offset + rows
	if end > len(filtered) {
		end = len(filtered)
	}

	for i := m.offset; i < end; i++ {
		r := filtered[i]
		selected := i == m.cursor
		alt := i%2 == 1
		line := m.renderRecord(r, colType, colName, colContent, colTTL, colProxy, selected, alt)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m Model) renderRecord(r cloudflare.DNSRecord, colType, colName, colContent, colTTL, colProxy int, selected, alt bool) string {
	typeStr := styleType(r.Type)
	name := truncateAnsi(r.Name, colName)
	content := truncateAnsi(r.Content, colContent)
	ttl := ttlDisplay(r.TTL)
	proxy := styleProxyField(r.Proxied)

	line := m.renderRow(typeStr, name, content, ttl, proxy,
		colType, colName, colContent, colTTL, colProxy, false, false)

	switch {
	case selected:
		return styleRowSelected.Render(line)
	case alt:
		return styleRowAlt.Render(line)
	default:
		return styleRowNormal.Render(line)
	}
}

func (m Model) renderRow(typ, name, content, ttl, proxy string, colType, colName, colContent, colTTL, colProxy int, header, _ bool) string {
	if header {
		s := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"})
		typ = s.Render(typ)
		name = s.Render(name)
		content = s.Render(content)
		ttl = s.Render(ttl)
		proxy = s.Render(proxy)
	}
	return padAnsi(typ, colType) + " " +
		padAnsi(name, colName) + " " +
		padAnsi(content, colContent) + " " +
		padAnsi(ttl, colTTL) + " " +
		proxy
}

func (m Model) renderStatusBar() string {
	filtered := m.filtered()
	total := len(m.records)
	shown := len(filtered)
	keys := "  [j/k] move  [n/p] zone  [/] filter  [r] reload  [q] quit"
	return styleStatus.Render(fmt.Sprintf("%d / %d records%s", shown, total, keys))
}

func (m Model) visibleRows() int {
	// Title + zones + filter + optional error + header + status
	reserved := 7
	if m.err != "" {
		reserved++
	}
	v := m.height - reserved
	if v < 1 {
		return 1
	}
	return v
}

func (m Model) filtered() []cloudflare.DNSRecord {
	q := strings.ToLower(strings.TrimSpace(m.filter))
	if q == "" {
		return m.records
	}
	out := make([]cloudflare.DNSRecord, 0, len(m.records))
	for _, r := range m.records {
		if strings.Contains(strings.ToLower(r.Name), q) ||
			strings.Contains(strings.ToLower(r.Type), q) ||
			strings.Contains(strings.ToLower(r.Content), q) {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) fetchRecords(idx int) tea.Cmd {
	m.zoneIdx = idx
	m.loading = true
	m.err = ""
	m.loadSeq++
	seq := m.loadSeq
	zone := m.zones[idx]
	client := m.client
	return func() tea.Msg {
		recs, err := client.Records(context.Background(), zone.ID)
		return recordsLoadedMsg{seq: seq, records: recs, err: err}
	}
}

func loadZones(client *cloudflare.Client) tea.Cmd {
	return func() tea.Msg {
		zones, err := client.Zones(context.Background())
		return zonesLoadedMsg{zones: zones, err: err}
	}
}

func spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

// truncateAnsi truncates a string to n visible runes, appending "…" if needed.
// ANSI escape codes are not counted toward the width.
func truncateAnsi(s string, n int) string {
	if n <= 0 {
		return ""
	}
	visible := 0
	for i, r := range s {
		if visible == n-1 && i < len(s)-1 {
			return s[:i] + "…"
		}
		if r == '\x1b' {
			continue
		}
		visible++
	}
	return s
}

// padAnsi right-pads s to width w using visible rune count.
func padAnsi(s string, w int) string {
	vis := visibleLen(s)
	if vis >= w {
		return truncateAnsi(s, w)
	}
	return s + strings.Repeat(" ", w-vis)
}

func visibleLen(s string) int {
	count := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		count++
	}
	return count
}
