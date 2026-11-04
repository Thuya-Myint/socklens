class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.8.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.8.0/socklens_v0.8.0_darwin_amd64.tar.gz"
      sha256 "c0e6e52c6778d2bd65844f63d5da2a9ea5d6d024f77b1881d3754ac0b4b5d191"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.8.0/socklens_v0.8.0_darwin_arm64.tar.gz"
      sha256 "4bbd11352235daa1a83d865298e4a23998441f105a7e65d0473bf40b1f2926c7"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.8.0/socklens_v0.8.0_linux_amd64.tar.gz"
      sha256 "bf273d698889bfc3a695fbf13a57dd192e92ea99c0267ebd2ce8757d1ad12224"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.8.0/socklens_v0.8.0_linux_arm64.tar.gz"
      sha256 "905132953d35d0568b21c3ae983cd831f809fe65597c3ed81ebf87d137841cdd"
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
