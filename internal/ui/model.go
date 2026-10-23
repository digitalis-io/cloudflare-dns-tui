package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/digitalis-io/cloudflare-dns-tui/internal/cloudflare"
)

// appMode is the current UI mode.
type appMode int

const (
	modeList appMode = iota
	modeForm
	modeConfirm
	modeZonePicker
)

// confirmAction is the pending operation the confirm dialog is asking about.
type confirmAction int

const (
	actionDelete confirmAction = iota
	actionCreate
	actionUpdate
)

const (
	fieldType    = 0
	fieldName    = 1
	fieldContent = 2
	fieldTTL     = 3
	fieldProxied = 4
	fieldCount   = 5
)

var fieldLabels = [fieldCount]string{"Type", "Name", "Content", "TTL", "Proxied"}

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

// recordSavedMsg is sent when a create/update API call completes.
type recordSavedMsg struct{ err error }

// recordDeletedMsg is sent when a delete API call completes.
type recordDeletedMsg struct{ err error }

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

	mode          appMode
	formFields    [fieldCount]string
	formFocus     int
	formIsEdit    bool
	formEditID    string
	confirmFocus  int // 0 = Cancel (safe), 1 = action
	confirmOp     confirmAction
	pendingRecord cloudflare.DNSRecord
	zoneCursor    int // cursor inside zone picker
}

// New creates a ready-to-run Model.
func New(client *cloudflare.Client) Model {
	return Model{client: client, width: 120, height: 30, loading: true}
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
		if m.loading {
			return m, spinnerTick()
		}
		return m, nil

	case zonesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.zones = msg.zones
		m.err = ""
		if len(m.zones) > 0 {
			return m.fetchRecords(0)
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

	case recordSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		return m.fetchRecords(m.zoneIdx)

	case recordDeletedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		return m.fetchRecords(m.zoneIdx)

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.InterruptMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeForm:
		return m.handleFormKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeZonePicker:
		return m.handleZonePickerKey(msg)
	}

	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	switch msg.Code {
	case tea.KeyEsc, 'q':
		return m, tea.Quit
	case 'c':
		if msg.Mod == tea.ModCtrl {
			return m, tea.Quit
		}

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
			return m.fetchRecords(idx)
		}

	case tea.KeyRight, 'n':
		if len(m.zones) > 1 && !m.loading {
			idx := (m.zoneIdx + 1) % len(m.zones)
			return m.fetchRecords(idx)
		}

	case 'r':
		if !m.loading {
			if len(m.zones) > 0 {
				return m.fetchRecords(m.zoneIdx)
			}
			m.loading = true
			return m, tea.Batch(loadZones(m.client), spinnerTick())
		}

	case '/':
		if !m.loading {
			m.filterMode = true
			return m, nil
		}

	case 'a':
		if !m.loading && len(m.zones) > 0 {
			m.formFields = [fieldCount]string{"A", "", "", "1", "no"}
			m.formFocus = fieldName
			m.formIsEdit = false
			m.formEditID = ""
			m.mode = modeForm
		}

	case 'e':
		if !m.loading {
			filtered := m.filtered()
			if len(filtered) > 0 && m.cursor < len(filtered) {
				r := filtered[m.cursor]
				ttlStr := "1"
				if r.TTL != 1 {
					ttlStr = strconv.Itoa(r.TTL)
				}
				proxiedStr := "no"
				if r.Proxied {
					proxiedStr = "yes"
				}
				m.formFields = [fieldCount]string{r.Type, r.Name, r.Content, ttlStr, proxiedStr}
				m.formFocus = fieldContent
				m.formIsEdit = true
				m.formEditID = r.ID
				m.mode = modeForm
			}
		}

	case 'd':
		if !m.loading {
			filtered := m.filtered()
			if len(filtered) > 0 && m.cursor < len(filtered) {
				m.pendingRecord = filtered[m.cursor]
				m.confirmOp = actionDelete
				m.confirmFocus = 0
				m.mode = modeConfirm
			}
		}

	case 'z':
		if len(m.zones) > 3 && !m.loading {
			m.zoneCursor = m.zoneIdx
			m.mode = modeZonePicker
		}
	}
	return m, nil
}

func (m Model) handleZonePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyUp, 'k':
		if m.zoneCursor > 0 {
			m.zoneCursor--
		}
	case tea.KeyDown, 'j':
		if m.zoneCursor < len(m.zones)-1 {
			m.zoneCursor++
		}
	case tea.KeyEnter:
		m.mode = modeList
		return m.fetchRecords(m.zoneCursor)
	case tea.KeyEsc, 'q':
		m.mode = modeList
	}
	return m, nil
}

func (m Model) handleFormKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEsc:
		m.mode = modeList
		m.err = ""
		return m, nil

	case tea.KeyEnter:
		if m.formFocus == fieldCount-1 {
			return m.submitForm()
		}
		m.formFocus = (m.formFocus + 1) % fieldCount

	case tea.KeyTab:
		m.formFocus = (m.formFocus + 1) % fieldCount

	case tea.KeyBackspace:
		if m.formFocus != fieldProxied && len(m.formFields[m.formFocus]) > 0 {
			f := m.formFields[m.formFocus]
			_, size := utf8.DecodeLastRuneInString(f)
			m.formFields[m.formFocus] = f[:len(f)-size]
		}

	default:
		if m.formFocus == fieldProxied {
			if msg.Text == " " || msg.Text == "y" || msg.Text == "n" {
				if m.formFields[fieldProxied] == "yes" {
					m.formFields[fieldProxied] = "no"
				} else {
					m.formFields[fieldProxied] = "yes"
				}
			}
		} else if msg.Text != "" {
			m.formFields[m.formFocus] += msg.Text
		}
	}
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyTab, tea.KeyLeft, tea.KeyRight, 'h', 'l':
		m.confirmFocus = 1 - m.confirmFocus

	case tea.KeyEnter:
		if m.confirmFocus == 1 {
			return m.execConfirmedAction()
		}
		m.mode = modeList
		m.confirmFocus = 0

	case tea.KeyEsc, 'n', 'N', 'q':
		m.mode = modeList
		m.confirmFocus = 0
	}
	return m, nil
}

func (m Model) execConfirmedAction() (tea.Model, tea.Cmd) {
	zone := m.zones[m.zoneIdx]
	rec := m.pendingRecord
	m.mode = modeList
	m.confirmFocus = 0
	m.loading = true
	client := m.client
	switch m.confirmOp {
	case actionDelete:
		return m, tea.Batch(func() tea.Msg {
			return recordDeletedMsg{err: client.DeleteRecord(context.Background(), zone.ID, rec.ID)}
		}, spinnerTick())
	case actionCreate:
		return m, tea.Batch(func() tea.Msg {
			_, err := client.CreateRecord(context.Background(), zone.ID, rec)
			return recordSavedMsg{err: err}
		}, spinnerTick())
	case actionUpdate:
		return m, tea.Batch(func() tea.Msg {
			return recordSavedMsg{err: client.UpdateRecord(context.Background(), zone.ID, rec)}
		}, spinnerTick())
	default:
		m.loading = false
		return m, nil
	}
}

func (m Model) submitForm() (tea.Model, tea.Cmd) {
	rec, err := m.formToRecord()
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	if m.formIsEdit {
		rec.ID = m.formEditID
		m.confirmOp = actionUpdate
	} else {
		m.confirmOp = actionCreate
	}
	m.pendingRecord = rec
	m.confirmFocus = 0
	m.mode = modeConfirm
	return m, nil
}

func (m Model) formToRecord() (cloudflare.DNSRecord, error) {
	recType := strings.ToUpper(strings.TrimSpace(m.formFields[fieldType]))
	if recType == "" {
		return cloudflare.DNSRecord{}, fmt.Errorf("type is required")
	}
	name := strings.TrimSpace(m.formFields[fieldName])
	if name == "" {
		return cloudflare.DNSRecord{}, fmt.Errorf("name is required")
	}
	content := strings.TrimSpace(m.formFields[fieldContent])
	if content == "" {
		return cloudflare.DNSRecord{}, fmt.Errorf("content is required")
	}
	ttl := 1
	ttlStr := strings.TrimSpace(m.formFields[fieldTTL])
	if ttlStr != "" && ttlStr != "1" && strings.ToLower(ttlStr) != "auto" {
		v, err := strconv.Atoi(ttlStr)
		if err != nil || v < 1 {
			return cloudflare.DNSRecord{}, fmt.Errorf("TTL must be a positive integer or 1 for auto")
		}
		ttl = v
	}
	proxied := m.formFields[fieldProxied] == "yes"
	return cloudflare.DNSRecord{Type: recType, Name: name, Content: content, TTL: ttl, Proxied: proxied}, nil
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

	if m.mode == modeForm {
		b.WriteString(m.renderForm())
		return b.String()
	}

	b.WriteString(m.renderFilterBar())
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString(styleError.Render("✗ " + m.err))
		b.WriteString("\n")
	}

	b.WriteString(m.renderTable())
	b.WriteString(m.renderStatusBar())

	bg := b.String()
	switch m.mode {
	case modeConfirm:
		return overlayCenter(bg, m.renderConfirmDialog(), m.width, m.height)
	case modeZonePicker:
		picker := m.renderZonePicker()
		// position below title bar + zone bar (row 2), left-aligned
		return overlayAt(bg, picker, 0, 2)
	}
	return bg
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
	if len(m.zones) > 3 {
		name := m.zones[m.zoneIdx].Name
		counter := styleStatus.Render(fmt.Sprintf(" %d/%d", m.zoneIdx+1, len(m.zones)))
		btn := styleZoneTabActive.Render("  " + name + " ▼  ")
		hint := styleStatus.Render("  ←/→ switch  [z] pick")
		return btn + counter + hint
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

func (m Model) renderZonePicker() string {
	maxW := 0
	for _, z := range m.zones {
		if ansi.StringWidth(z.Name) > maxW {
			maxW = ansi.StringWidth(z.Name)
		}
	}
	w := maxW + 6

	var lines []string
	for i, z := range m.zones {
		label := padAnsi(z.Name, maxW)
		marker := "  "
		if i == m.zoneCursor {
			marker = "> "
		}
		entry := marker + label + "  "
		switch i {
		case m.zoneCursor:
			lines = append(lines, styleZonePickerActive.Render(entry))
		case m.zoneIdx:
			lines = append(lines, styleZonePickerCurrent.Render(entry))
		default:
			lines = append(lines, styleZonePickerItem.Render(entry))
		}
	}

	hint := lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Width(w).
		Align(lipgloss.Center).Render("↑↓ move  Enter select  Esc close")

	return styleZonePickerBox.Width(w).Render(strings.Join(lines, "\n") + "\n\n" + hint)
}

func (m Model) renderFilterBar() string {
	label := "Filter: "
	var content string
	switch {
	case m.filterMode:
		content = styleFilter.Render(label + m.filter + "█")
	case m.filter != "":
		content = styleFilterInactive.Render(label + m.filter + "  [/ then esc]")
	default:
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
		colType, colName, colContent, colTTL, true)
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
		line := m.renderRecord(r, colType, colName, colContent, colTTL, selected, alt)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m Model) renderRecord(r cloudflare.DNSRecord, colType, colName, colContent, colTTL int, selected, alt bool) string {
	var typeStr, proxy string
	if selected {
		typeStr = r.Type
		if r.Proxied {
			proxy = "☁ Proxied"
		} else {
			proxy = "DNS only"
		}
	} else {
		typeStr = styleType(r.Type)
		proxy = styleProxyField(r.Proxied)
	}
	name := ansi.Truncate(r.Name, colName, "…")
	content := ansi.Truncate(r.Content, colContent, "…")
	ttl := ttlDisplay(r.TTL)

	line := m.renderRow(typeStr, name, content, ttl, proxy,
		colType, colName, colContent, colTTL, false)

	switch {
	case selected:
		return styleRowSelected.Render(line)
	case alt:
		return styleRowAlt.Render(line)
	default:
		return styleRowNormal.Render(line)
	}
}

func (m Model) renderRow(typ, name, content, ttl, proxy string, colType, colName, colContent, colTTL int, header bool) string {
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
	keys := "  [j/k] move  [n/p] zone  [/] filter  [a] add  [e] edit  [d] delete  [r] reload  [q] quit"
	return styleStatus.Render(fmt.Sprintf("%d / %d records%s", shown, total, keys))
}

func (m Model) renderForm() string {
	title := "Add Record"
	if m.formIsEdit {
		title = "Edit Record"
	}

	fieldWidth := m.width - 16
	if fieldWidth < 20 {
		fieldWidth = 20
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleFormTitle.Render(fmt.Sprintf("  %s  ", title)))
	b.WriteString("\n\n")

	for i := 0; i < fieldCount; i++ {
		label := padAnsi(fieldLabels[i], 8)
		val := m.formFields[i]
		display := val
		if i == fieldTTL && (val == "1" || val == "") {
			display = "Auto"
		}
		cursor := " "
		if i == m.formFocus {
			cursor = "█"
		}
		fieldVal := padAnsi(display+cursor, fieldWidth)
		if i == m.formFocus {
			b.WriteString(styleFormLabelActive.Render(label) + styleFormFieldActive.Render(fieldVal))
		} else {
			b.WriteString(styleFormLabel.Render(label) + styleFormField.Render(fieldVal))
		}
		b.WriteString("\n")
	}

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(styleError.Render("✗ " + m.err))
	}
	b.WriteString("\n")
	b.WriteString(styleStatus.Render("  [Tab] next field  [Enter] submit  [Esc] cancel"))
	return b.String()
}

func (m Model) renderConfirmDialog() string {
	rec := m.pendingRecord
	recDesc := fmt.Sprintf("%s  %s  %s", rec.Type, rec.Name, rec.Content)

	var title, actionLabel string
	var titleColor, actionBg, actionFg lipgloss.TerminalColor
	switch m.confirmOp {
	case actionDelete:
		title, actionLabel = "⚠  Delete Record", "Delete"
		titleColor = colorError
		actionBg, actionFg = lipgloss.Color("#dc2626"), lipgloss.Color("#fff")
	case actionCreate:
		title, actionLabel = "Create Record", "Create"
		titleColor = colorSuccess
		actionBg, actionFg = lipgloss.Color("#16a34a"), lipgloss.Color("#fff")
	case actionUpdate:
		title, actionLabel = "Update Record", "Update"
		titleColor = colorSelected
		actionBg, actionFg = lipgloss.Color("#2563eb"), lipgloss.Color("#fff")
	}
	choices := []string{"Cancel", actionLabel}

	const keysLine = "← →/Tab: Move   Enter: Choose   Esc: Cancel"
	btnRow := dialogButtonRow(choices, m.confirmFocus)
	inner := 40
	for _, s := range []string{recDesc, keysLine, btnRow} {
		if w := lipgloss.Width(s); w > inner {
			inner = w
		}
	}

	centre := func(s string) string {
		gap := max(inner-lipgloss.Width(s), 0)
		return strings.Repeat(" ", gap/2) + s + strings.Repeat(" ", gap-gap/2)
	}

	titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)
	recStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"}).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	lines := []string{
		"",
		titleStyle.Render(centre(title)),
		"",
		recStyle.Render(centre(recDesc)),
		"",
		renderDialogButtons(choices, m.confirmFocus, inner, actionBg, actionFg),
		"",
		keyStyle.Render(centre(keysLine)),
		"",
	}

	return styleDialog.Padding(0, 2).Width(inner + 4).Render(strings.Join(lines, "\n"))
}

// dialogButtonRow returns the full button row string (used for width measurement).
func dialogButtonRow(choices []string, _ int) string {
	const gap = "     "
	parts := make([]string, len(choices))
	for i, c := range choices {
		parts[i] = "  [ " + c + " ]"
	}
	return strings.Join(parts, gap)
}

// renderDialogButtons renders centered dialog buttons, cqlai-style.
// choices[0] = Cancel (safe/green), choices[1] = action (colored per actionBg/actionFg).
func renderDialogButtons(choices []string, selected, inner int, actionBg, actionFg lipgloss.TerminalColor) string {
	const gap = "     "
	total := 0
	for i, c := range choices {
		total += 2 + 4 + len(c) // "  [ c ]"
		if i < len(choices)-1 {
			total += len(gap)
		}
	}
	left := max((inner-total)/2, 0)

	cancelFill := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#052e16")).
		Background(lipgloss.Color("#4ade80")).Bold(true)
	actionFill := lipgloss.NewStyle().
		Foreground(actionFg).Background(actionBg).Bold(true)

	var row strings.Builder
	row.WriteString(strings.Repeat(" ", left))
	for i, c := range choices {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		label := "[ " + c + " ]"
		if i == selected {
			fill := actionFill
			if i == 0 {
				fill = cancelFill
			}
			row.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(marker))
			row.WriteString(fill.Render(label))
		} else {
			row.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(marker + label))
		}
		if i < len(choices)-1 {
			row.WriteString(gap)
		}
	}
	return row.String()
}

// overlayCenter composites a dialog string centered over the background string.
func overlayCenter(background, dialog string, screenW, screenH int) string {
	dialogLines := strings.Split(dialog, "\n")
	dh := len(dialogLines)
	dw := 0
	for _, l := range dialogLines {
		if w := lipgloss.Width(l); w > dw {
			dw = w
		}
	}

	x := max((screenW-dw)/2, 0)
	y := max((screenH-dh)/2, 0)

	bgLines := strings.Split(background, "\n")
	for len(bgLines) < y+dh {
		bgLines = append(bgLines, "")
	}

	const reset = "\x1b[0m"
	for i, dLine := range dialogLines {
		li := y + i
		if li >= len(bgLines) {
			break
		}
		bg := bgLines[li]
		bgW := lipgloss.Width(bg)

		var out string
		if x > 0 {
			out = ansi.Cut(bg, 0, x)
			if pad := x - lipgloss.Width(out); pad > 0 {
				out += strings.Repeat(" ", pad)
			}
			out += reset
		}
		out += dLine
		if end := x + lipgloss.Width(dLine); end < bgW {
			out += reset + ansi.Cut(bg, end, bgW)
		}
		bgLines[li] = out
	}
	return strings.Join(bgLines, "\n")
}

// overlayAt composites an overlay string at a fixed (x, y) position over the background.
func overlayAt(background, overlay string, x, y int) string {
	overlayLines := strings.Split(overlay, "\n")
	bgLines := strings.Split(background, "\n")
	for len(bgLines) < y+len(overlayLines) {
		bgLines = append(bgLines, "")
	}
	const reset = "\x1b[0m"
	for i, oLine := range overlayLines {
		li := y + i
		if li >= len(bgLines) {
			break
		}
		bg := bgLines[li]
		bgW := lipgloss.Width(bg)
		oW := lipgloss.Width(oLine)
		var out string
		if x > 0 {
			out = ansi.Cut(bg, 0, x)
			if pad := x - lipgloss.Width(out); pad > 0 {
				out += strings.Repeat(" ", pad)
			}
			out += reset
		}
		out += oLine
		if end := x + oW; end < bgW {
			out += reset + ansi.Cut(bg, end, bgW)
		}
		bgLines[li] = out
	}
	return strings.Join(bgLines, "\n")
}

func (m Model) visibleRows() int {
	// Title + zones + filter (3 lines: top border, content, bottom border) + header + border + status
	reserved := 9
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

func (m Model) fetchRecords(idx int) (Model, tea.Cmd) {
	m.zoneIdx = idx
	m.loading = true
	m.err = ""
	m.loadSeq++
	seq := m.loadSeq
	zone := m.zones[idx]
	client := m.client
	cmd := func() tea.Msg {
		recs, err := client.Records(context.Background(), zone.ID)
		return recordsLoadedMsg{seq: seq, records: recs, err: err}
	}
	return m, tea.Batch(cmd, spinnerTick())
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

// padAnsi right-pads s to width w using terminal display width.
func padAnsi(s string, w int) string {
	vis := ansi.StringWidth(s)
	if vis >= w {
		return ansi.Truncate(s, w, "")
	}
	return s + strings.Repeat(" ", w-vis)
}
