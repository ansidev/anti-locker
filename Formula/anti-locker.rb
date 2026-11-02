class AntiLocker < Formula
  desc "Keep your Mac awake via caffeinate when connected to a trusted Wi-Fi network"
  homepage "https://github.com/ansidev/anti-locker"
  url "https://github.com/ansidev/anti-locker/archive/refs/tags/v1.0.3.tar.gz"
  sha256 "9a572ab5f76e1b35ecad1cdb32be32e43462a1627d4bdd841dd0fcf4a3440c56"
  license "Apache-2.0"
  head "https://github.com/ansidev/anti-locker.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "."
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/anti-locker --version")
  end
end
