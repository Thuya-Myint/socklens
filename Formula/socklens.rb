class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.3.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.3.0/socklens_v0.3.0_darwin_amd64.tar.gz"
      sha256 "7a8d891679725a577e81c4a37bd47110ea6b125b36227bce8b62547c5ff5b20f"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.3.0/socklens_v0.3.0_darwin_arm64.tar.gz"
      sha256 "01e8e00c70b3412cc76922545de7b7e40410895b19528f2f778ef14375edd1d4"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.3.0/socklens_v0.3.0_linux_amd64.tar.gz"
      sha256 "47ec8707152430df2a6b7640f2d2a867b9e5110f71d8ae099542263fd2a8e38c"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.3.0/socklens_v0.3.0_linux_arm64.tar.gz"
      sha256 "1faa0e368f2b3a0cf4507d694d403359a513d7cb439e36c6ee0f9c0603f0d807"
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
