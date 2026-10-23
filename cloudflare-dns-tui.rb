class CloudflareDnsTui < Formula
  desc "Cloudflare DNS TUI - terminal user interface for managing Cloudflare DNS"
  homepage "https://github.com/digitalis-io/cloudflare-dns-tui"
  url "https://github.com/digitalis-io/cloudflare-dns-tui.git",
      using: :git,
      tag:   "v0.1.0"
  license "Apache-2.0"
  head "https://github.com/digitalis-io/cloudflare-dns-tui.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "./cmd/cloudflare-dns-tui"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/cloudflare-dns-tui --version 2>&1", 1)
  end
end
