class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.9.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.0/socklens_v0.9.0_darwin_amd64.tar.gz"
      sha256 "86713efb03a7a8b04348e95026678ba1c2051e02f8ee8fda45512d7412863a76"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.0/socklens_v0.9.0_darwin_arm64.tar.gz"
      sha256 "6ca6dde79b18c08aa8e95c18b1e6ae3a2736e4292da2b96e2e8760d789e6da40"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.0/socklens_v0.9.0_linux_amd64.tar.gz"
      sha256 "07ef8e86e49e4ab3743355c705b1684337f518f9163416e1aac4f96225f008a6"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.0/socklens_v0.9.0_linux_arm64.tar.gz"
      sha256 "d26bab3cbd0ed4f859216453dade1722f97148fd92f67a4c04690fb4f40560e3"
    end
  end
  def install
    bin.install "socklens"
    doc.install "README.md", "CHANGELOG.md"
  end
  test do
    assert_match version.to_s, shell_output("#{bin}/socklens version")
  end
end
