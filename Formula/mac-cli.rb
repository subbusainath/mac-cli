class MacCli < Formula
  desc "My Agentic CLI — stateful LangGraph TDD coding assistant"
  homepage "https://github.com/subbusainath/mac-cli"
  url "https://github.com/subbusainath/mac-cli/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "66a2894ede6ac717ac11626bd39dd94fbed06e15049485bb0f9836b4468d3556"
  version "0.1.0"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"mac", ldflags: "-s -w -X main.version=#{version}"), "./cmd/mac/"
  end

  def caveats
    <<~EOS
      To use 'mac code', install the Python orchestrator:
        uv tool install --from /path/to/mac-cli/orchestrator mac-orchestrator

      PostgreSQL is required. Set MAC_DB_URL or use the default:
        postgres://postgres:postgres@localhost:5432/mac_cli?sslmode=disable
    EOS
  end

  test do
    assert_match "Agentic", shell_output("#{bin}/mac --help")
  end
end
