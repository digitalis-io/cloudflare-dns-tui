// Command cloudflare-dns-tui is a terminal UI for browsing Cloudflare DNS records.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/digitalis-io/cloudflare-dns-tui/internal/cloudflare"
	"github.com/digitalis-io/cloudflare-dns-tui/internal/ui"
)

func main() {
	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	client, err := cloudflare.NewClient(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: set CLOUDFLARE_API_TOKEN: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(ui.New(client))
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
