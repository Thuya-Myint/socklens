class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.5.1"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.1/socklens_v0.5.1_darwin_amd64.tar.gz"
      sha256 "42ae874d47af13c4ca1efa854f04cb9d1f4e2846d512380be102b8b4b8edd139"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.1/socklens_v0.5.1_darwin_arm64.tar.gz"
      sha256 "c4e56d8a78fc5bb5e0b53ce9f4fee0115e1255b54fd39034fe9b6dec7e59f916"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.1/socklens_v0.5.1_linux_amd64.tar.gz"
      sha256 "79b53f6e8ee390744b60df40f4661ef20fc237091b626c66ba976ebd9b6c69ef"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.1/socklens_v0.5.1_linux_arm64.tar.gz"
      sha256 "ff6ef8af141bbc3c789781691a3e3835a266c60bc015035fa498d68ce5a37c00"
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
