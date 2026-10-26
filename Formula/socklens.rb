class Socklens < Formula
  desc "Inspect and monitor local ports with confirmed process termination"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.1.0"
  license "MIT"

  depends_on :linux

  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.1.0/socklens_v0.1.0_linux_amd64.tar.gz"
      sha256 "93f237669a18ea92273c94f991e5250347c134ddb1c4517faaa5dad481306632"
    end

    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.1.0/socklens_v0.1.0_linux_arm64.tar.gz"
      sha256 "7527377b7c09d004608cd53eaa3b5d547e1c06e8c0da482580adf77264484650"
    end
  end

  def install
    bin.install "socklens"
    doc.install "README.md", "CHANGELOG.md"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/socklens version")
    assert_match "Usage:", shell_output("#{bin}/socklens help")
  end
end
