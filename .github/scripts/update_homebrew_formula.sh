#!/usr/bin/env bash
set -euo pipefail

VERSION="${TAG#v}"
URL="https://github.com/ansidev/anti-locker/archive/refs/tags/${TAG}.tar.gz"
echo "Downloading ${URL}"
SHA="$(curl -fsL "${URL}" | sha256sum | awk '{print $1}')"

mkdir -p Formula
cat > Formula/anti-locker.rb <<EOF
class AntiLocker < Formula
  desc "Keep your Mac awake via caffeinate when connected to a trusted Wi-Fi network"
  homepage "https://github.com/ansidev/anti-locker"
  url "${URL}"
  sha256 "${SHA}"
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
EOF
