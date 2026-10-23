<p align="center">
  <a href="https://digitalis.io">
    <img src="https://cdn.prod.website-files.com/68370d67a3c849c28c22f703/68370d67a3c849c28c22f803_digitalis-logo.svg" alt="Digitalis.io" width="200" />
  </a>
</p>

<h1 align="center">cloudflare-dns-tui</h1>
<p align="center">A terminal UI for browsing and managing Cloudflare DNS records.</p>

---

## Overview

`cloudflare-dns-tui` is a keyboard-driven terminal application built with [Bubbletea v2](https://charm.land/bubbletea). It lets you list, create, update, and delete DNS records across all your Cloudflare zones without leaving the terminal.

## Requirements

- Go 1.23+
- A [Cloudflare API token](https://dash.cloudflare.com/profile/api-tokens) with `Zone:Read` and `DNS:Edit` permissions

## Quick Start

**Via Homebrew (from source):**

```bash
brew install --build-from-source ./cloudflare-dns-tui.rb
export CLOUDFLARE_API_TOKEN=your_token_here
cloudflare-dns-tui
```

**Via Go:**

```bash
export CLOUDFLARE_API_TOKEN=your_token_here
go run ./cmd/cloudflare-dns-tui
```

Or build and run:

```bash
make build
export CLOUDFLARE_API_TOKEN=your_token_here
./bin/cloudflare-dns-tui
```

## Usage

| Key | Action |
|-----|--------|
| `↑` / `↓` or `k` / `j` | Navigate records |
| `←` / `→` or `p` / `n` | Switch zone |
| `z` | Zone picker (when >3 zones) |
| `/` | Filter records |
| `a` | Add new record |
| `e` | Edit selected record |
| `d` | Delete selected record |
| `r` | Reload records |
| `q` / `Ctrl+C` | Quit |

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `CLOUDFLARE_API_TOKEN` | Yes | Cloudflare API token with Zone:Read and DNS:Edit scopes |

## Build

```bash
make build    # compile to ./bin/cloudflare-dns-tui
make test     # unit tests
make lint     # golangci-lint
make fmt      # gofmt -s -w .
```

## Development

```bash
make help     # list all targets
make cover    # coverage report
make vet      # go vet
make tidy     # go mod tidy
```

---

<p align="center">
  Maintained by <a href="https://digitalis.io">Digitalis.io</a> &mdash;
  <em>We help companies deliver and operate data anywhere</em>
  <br />
  <a href="https://digitalis.io/contact">digitalis.io/contact</a>
</p>
