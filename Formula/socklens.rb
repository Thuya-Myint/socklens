class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.6.1"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.1/socklens_v0.6.1_darwin_amd64.tar.gz"
      sha256 "2a2df0bfe217a078158170e24e6835ef0b02405504cccac7b42ef9b450c43d58"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.1/socklens_v0.6.1_darwin_arm64.tar.gz"
      sha256 "b7792ffed93f24c1727f5efd0d2dd6faecb94797f335aa1885ac5eaec68a57d5"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.1/socklens_v0.6.1_linux_amd64.tar.gz"
      sha256 "03c915ee5dc794ea3bb55358737ed9fa8a992ea42d757cc69fa503e7e9fb2458"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.1/socklens_v0.6.1_linux_arm64.tar.gz"
      sha256 "58c75c521ef89ce1cd5c2e133ae7bb41a98c7194f391add231fa380e80f2fa0a"
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
