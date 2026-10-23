package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/digitalis-io/cloudflare-dns-tui/internal/cloudflare"
)

func sendKey(m Model, code rune, text string) (Model, tea.Cmd) {
	nm, cmd := m.Update(tea.KeyPressMsg{Code: code, Text: text})
	return nm.(Model), cmd
}

func TestModel_QuitOnQ(t *testing.T) {
	m := New(nil)
	m.loading = false
	_, cmd := sendKey(m, 'q', "q")
	if cmd == nil {
		t.Fatal("want Quit cmd, got nil")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("cmd returned nil msg")
	}
}

func TestModel_QuitOnCtrlC(t *testing.T) {
	m := New(nil)
	m.loading = false
	nm, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	_ = nm
	if cmd == nil {
		t.Fatal("want Quit cmd from ctrl+c")
	}
}

func TestModel_FilterMode(t *testing.T) {
	m := New(nil)
	m.loading = false
	m.records = []cloudflare.DNSRecord{
		{Type: "A", Name: "alpha.example.com", Content: "1.2.3.4"},
		{Type: "CNAME", Name: "beta.example.com", Content: "alpha.example.com"},
	}

	m, _ = sendKey(m, '/', "/")
	if !m.filterMode {
		t.Fatal("want filterMode=true after /")
	}

	m, _ = sendKey(m, 'a', "a")
	m, _ = sendKey(m, 'l', "l")
	if m.filter != "al" {
		t.Errorf("want filter='al', got %q", m.filter)
	}

	filtered := m.filtered()
	if len(filtered) != 2 {
		t.Errorf("want 2 matching records, got %d", len(filtered))
	}

	m, _ = sendKey(m, tea.KeyEsc, "")
	if m.filterMode {
		t.Fatal("want filterMode=false after esc")
	}
	if m.filter != "" {
		t.Errorf("want empty filter after esc, got %q", m.filter)
	}
}

func TestModel_FilterNoMatch(t *testing.T) {
	m := New(nil)
	m.loading = false
	m.records = []cloudflare.DNSRecord{
		{Type: "A", Name: "example.com", Content: "1.2.3.4"},
	}
	m.filterMode = true
	m.filter = "zzz"

	filtered := m.filtered()
	if len(filtered) != 0 {
		t.Errorf("want 0 matching records, got %d", len(filtered))
	}
}

func TestModel_CursorNavigation(t *testing.T) {
	m := New(nil)
	m.loading = false
	m.records = makeRecords(5)

	m, _ = sendKey(m, 'j', "")
	if m.cursor != 1 {
		t.Errorf("cursor want 1, got %d", m.cursor)
	}

	m, _ = sendKey(m, 'k', "")
	if m.cursor != 0 {
		t.Errorf("cursor want 0 after k, got %d", m.cursor)
	}

	m, _ = sendKey(m, 'k', "")
	if m.cursor != 0 {
		t.Error("cursor should not go below 0")
	}
}

func TestModel_LoadSeqGuard(t *testing.T) {
	m := New(nil)
	m.loading = true
	m.zones = []cloudflare.Zone{{ID: "z1", Name: "a.example.com"}, {ID: "z2", Name: "b.example.com"}}
	m.loadSeq = 2

	stale := recordsLoadedMsg{seq: 1, records: makeRecords(3)}
	nm, _ := m.Update(stale)
	nm2 := nm.(Model)
	if len(nm2.records) != 0 {
		t.Error("stale recordsLoadedMsg should be ignored")
	}

	current := recordsLoadedMsg{seq: 2, records: makeRecords(3)}
	nm3, _ := nm2.Update(current)
	nm4 := nm3.(Model)
	if len(nm4.records) != 3 {
		t.Errorf("current msg want 3 records, got %d", len(nm4.records))
	}
}

func TestModel_WindowResize(t *testing.T) {
	m := New(nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m2 := nm.(Model)
	if m2.width != 200 || m2.height != 50 {
		t.Errorf("want 200x50, got %dx%d", m2.width, m2.height)
	}
}

func TestModel_SpinnerTick(t *testing.T) {
	m := New(nil)
	m.spinnerFrame = 0
	nm, _ := m.Update(spinnerTickMsg{})
	m2 := nm.(Model)
	if m2.spinnerFrame != 1 {
		t.Errorf("spinner frame want 1, got %d", m2.spinnerFrame)
	}
}

func TestModel_ViewContainsTitleAndStatus(t *testing.T) {
	m := New(nil)
	m.width = 120
	m.height = 40
	m.loading = false
	m.zones = []cloudflare.Zone{{ID: "z1", Name: "example.com"}}
	m.zoneIdx = 0
	m.records = makeRecords(2)

	v := m.View()
	content := v.Content
	if !strings.Contains(content, "Cloudflare DNS") {
		t.Error("view missing title")
	}
	if !strings.Contains(content, "example.com") {
		t.Error("view missing zone name")
	}
}

func TestModel_FilterBackspace(t *testing.T) {
	m := New(nil)
	m.filterMode = true
	m.filter = "abc"

	m, _ = sendKey(m, tea.KeyBackspace, "")
	if m.filter != "ab" {
		t.Errorf("want 'ab' after backspace, got %q", m.filter)
	}
}

func TestModel_ZonesLoadedError(t *testing.T) {
	m := New(nil)
	m.loading = true
	nm, _ := m.Update(zonesLoadedMsg{err: fmt.Errorf("auth failed")})
	m2 := nm.(Model)
	if m2.err != "auth failed" {
		t.Errorf("want err='auth failed', got %q", m2.err)
	}
	if m2.loading {
		t.Error("loading should be false after error")
	}
}

func TestModel_InitReturnsCmds(t *testing.T) {
	m := New(nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should return a non-nil Cmd")
	}
}

func TestModel_InterruptMsg(t *testing.T) {
	m := New(nil)
	_, cmd := m.Update(tea.InterruptMsg{})
	if cmd == nil {
		t.Fatal("want Quit cmd from InterruptMsg")
	}
}

func TestModel_ZoneNavigation(t *testing.T) {
	m := New(nil)
	m.loading = false
	m.zones = []cloudflare.Zone{
		{ID: "z1", Name: "alpha.example.com"},
		{ID: "z2", Name: "beta.example.com"},
		{ID: "z3", Name: "gamma.example.com"},
	}
	m.zoneIdx = 0
	m.records = makeRecords(2)

	// n = next zone
	m2, cmd := sendKey(m, 'n', "")
	if cmd == nil {
		t.Fatal("want cmd for zone change")
	}
	if m2.zoneIdx != 1 {
		t.Errorf("want zoneIdx=1, got %d", m2.zoneIdx)
	}

	// p = prev zone wraps
	m3, _ := sendKey(m, 'p', "")
	if m3.zoneIdx != 2 {
		t.Errorf("want zoneIdx=2 (wrap), got %d", m3.zoneIdx)
	}
}

func TestModel_Reload(t *testing.T) {
	m := New(nil)
	m.loading = false
	m.zones = []cloudflare.Zone{{ID: "z1", Name: "example.com"}}
	m.zoneIdx = 0
	m.records = makeRecords(3)

	m2, cmd := sendKey(m, 'r', "r")
	if cmd == nil {
		t.Fatal("want cmd for reload")
	}
	if !m2.loading {
		t.Error("want loading=true after r")
	}
}

func TestModel_ReloadNoZones(t *testing.T) {
	m := New(nil)
	m.loading = false
	m.zones = nil

	m2, cmd := sendKey(m, 'r', "r")
	if cmd == nil {
		t.Fatal("want cmd for reload when no zones")
	}
	if !m2.loading {
		t.Error("want loading=true after r with no zones")
	}
}

func TestModel_FilterEnterExits(t *testing.T) {
	m := New(nil)
	m.filterMode = true
	m.filter = "test"

	m2, _ := sendKey(m, tea.KeyEnter, "")
	if m2.filterMode {
		t.Error("want filterMode=false after enter")
	}
	if m2.filter != "test" {
		t.Errorf("want filter preserved after enter, got %q", m2.filter)
	}
}

func TestModel_ViewWithLoading(t *testing.T) {
	m := New(nil)
	m.width = 120
	m.height = 40
	m.loading = true

	v := m.View()
	if !strings.Contains(v.Content, spinnerFrames[m.spinnerFrame]) {
		t.Error("view should contain spinner when loading")
	}
}

func TestModel_ViewWithError(t *testing.T) {
	m := New(nil)
	m.width = 120
	m.height = 40
	m.loading = false
	m.err = "network timeout"

	v := m.View()
	if !strings.Contains(v.Content, "network timeout") {
		t.Error("view should contain error message")
	}
}

func TestModel_ViewWithFilter(t *testing.T) {
	m := New(nil)
	m.width = 120
	m.height = 40
	m.filter = "myfilter"

	v := m.View()
	if !strings.Contains(v.Content, "myfilter") {
		t.Error("view should show active filter")
	}
}

func TestModel_ViewAltScreenEnabled(t *testing.T) {
	m := New(nil)
	v := m.View()
	if !v.AltScreen {
		t.Error("view should enable AltScreen")
	}
}

func TestModel_CursorOffsetScrollDown(t *testing.T) {
	m := New(nil)
	m.width = 120
	m.height = 15
	m.records = makeRecords(20)
	m.cursor = 0
	m.offset = 0

	// scroll down past visible rows
	for i := 0; i < 12; i++ {
		m, _ = sendKey(m, 'j', "")
	}
	if m.cursor != 12 {
		t.Errorf("cursor want 12, got %d", m.cursor)
	}
	if m.offset == 0 {
		t.Error("offset should have advanced beyond 0")
	}
}

func TestModel_ZonesLoadedSuccess(t *testing.T) {
	m := New(nil)
	m.loading = true
	zones := []cloudflare.Zone{{ID: "z1", Name: "example.com"}}

	// First zones message triggers record fetch
	nm, cmd := m.Update(zonesLoadedMsg{zones: zones})
	m2 := nm.(Model)
	if len(m2.zones) != 1 {
		t.Errorf("want 1 zone, got %d", len(m2.zones))
	}
	if cmd == nil {
		t.Fatal("want cmd to load records after zones")
	}
}

func TestModel_ZonesLoadedEmpty(t *testing.T) {
	m := New(nil)
	m.loading = true

	nm, cmd := m.Update(zonesLoadedMsg{zones: nil})
	m2 := nm.(Model)
	if len(m2.zones) != 0 {
		t.Errorf("want 0 zones, got %d", len(m2.zones))
	}
	_ = cmd
}

func TestTTLDisplay(t *testing.T) {
	if ttlDisplay(1) != "Auto" {
		t.Errorf("ttl 1 want 'Auto', got %q", ttlDisplay(1))
	}
	if ttlDisplay(300) != "300" {
		t.Errorf("ttl 300 want '300', got %q", ttlDisplay(300))
	}
}

func TestStyleType(t *testing.T) {
	s := styleType("A")
	if s == "" {
		t.Error("styleType should return non-empty string")
	}
	s2 := styleType("UNKNOWN")
	if s2 == "" {
		t.Error("styleType with unknown type should return non-empty string")
	}
}

func TestStyleProxyField(t *testing.T) {
	p := styleProxyField(true)
	if !strings.Contains(p, "Proxied") {
		t.Errorf("proxied=true want 'Proxied', got %q", p)
	}
	d := styleProxyField(false)
	if !strings.Contains(d, "DNS only") {
		t.Errorf("proxied=false want 'DNS only', got %q", d)
	}
}

func makeRecords(n int) []cloudflare.DNSRecord {
	recs := make([]cloudflare.DNSRecord, n)
	for i := range recs {
		recs[i] = cloudflare.DNSRecord{
			Type:    "A",
			Name:    "host" + itoa(i) + ".example.com",
			Content: "1.2.3." + itoa(i),
			TTL:     300,
		}
	}
	return recs
}
