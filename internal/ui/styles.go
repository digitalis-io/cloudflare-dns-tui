// Package ui provides the bubbletea model and rendering for the DNS TUI.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorBg       = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1A1A2E"}
	colorBgAlt    = lipgloss.AdaptiveColor{Light: "#F1F5F9", Dark: "#16213E"}
	colorHeader   = lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"}
	colorMuted    = lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#94A3B8"}
	colorBorder   = lipgloss.AdaptiveColor{Light: "#CBD5E1", Dark: "#334155"}
	colorSelected = lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"}
	colorError    = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"}
	colorSuccess  = lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#4ADE80"}

	// Cloudflare orange for proxied indicator.
	colorCFOrange = lipgloss.Color("#F38020")

	styleTitleBar = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#F38020")).
			PaddingLeft(2).
			PaddingRight(2)

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#1E293B", Dark: "#E2E8F0"}).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder)

	styleRowNormal = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#CBD5E1"})

	styleRowAlt = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#334155", Dark: "#CBD5E1"}).
			Background(lipgloss.AdaptiveColor{Light: "#F8FAFC", Dark: "#0F172A"})

	styleRowSelected = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1D3461"}).
				Bold(true)

	styleZoneTab = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(colorMuted)

	styleZoneTabActive = lipgloss.NewStyle().
				Padding(0, 2).
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#F38020"))

	styleFilter = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSelected)

	styleFilterInactive = lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)

	styleStatus = lipgloss.NewStyle().
			Foreground(colorMuted).
			PaddingLeft(1)

	styleError = lipgloss.NewStyle().
			Foreground(colorError).
			PaddingLeft(1)

	styleSpinner = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F38020"))

	styleProxied = lipgloss.NewStyle().
			Foreground(colorCFOrange)

	styleDNSOnly = lipgloss.NewStyle().
			Foreground(colorMuted)

	// typeColors maps DNS record types to distinct colours.
	typeColors = map[string]lipgloss.Color{
		"A":      "#4ADE80",
		"AAAA":   "#22D3EE",
		"CNAME":  "#FB923C",
		"MX":     "#A78BFA",
		"TXT":    "#FDE68A",
		"NS":     "#F9A8D4",
		"SRV":    "#67E8F9",
		"CAA":    "#FCA5A5",
		"PTR":    "#86EFAC",
		"SOA":    "#C4B5FD",
		"DS":     "#FDA4AF",
		"DNSKEY": "#BAE6FD",
	}

	colorTypeDefault = lipgloss.Color("#94A3B8")
)

// typeColor returns the lipgloss color for a given DNS record type.
func typeColor(t string) lipgloss.Color {
	if c, ok := typeColors[t]; ok {
		return c
	}
	return colorTypeDefault
}

// styleType returns a styled DNS type badge.
func styleType(t string) string {
	return lipgloss.NewStyle().
		Foreground(typeColor(t)).
		Bold(true).
		Render(t)

}

// styleProxyField renders the proxy field with appropriate color.
func styleProxyField(proxied bool) string {
	if proxied {
		return styleProxied.Render("☁ Proxied")
	}
	return styleDNSOnly.Render("DNS only")
}

// ttlDisplay converts a raw TTL value to a human-readable string.
func ttlDisplay(ttl int) string {
	if ttl == 1 {
		return "Auto"
	}
	return itoa(ttl)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte(n%10) + '0'
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
