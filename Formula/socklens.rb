class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.4.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.4.0/socklens_v0.4.0_darwin_amd64.tar.gz"
      sha256 "6a8e1ef54f40cca856deef4e467a9d71e9080d9834287c524dfc16d50bef1995"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.4.0/socklens_v0.4.0_darwin_arm64.tar.gz"
      sha256 "6a2038132f0cc97d5ec8870fa56f3487bc45fe5234ae0d47c2ee2198f29e3fcc"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.4.0/socklens_v0.4.0_linux_amd64.tar.gz"
      sha256 "6652d3aa4d57dcb39eebd97627994b0986a53e964ae5200d871a45b6b89d9dd2"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.4.0/socklens_v0.4.0_linux_arm64.tar.gz"
      sha256 "695acc0d7f478249a84fac19764d5da45a79cbfd610b24722e6213fb9aaa5ebf"
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
