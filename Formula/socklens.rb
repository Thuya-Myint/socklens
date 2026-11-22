class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.12.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.0/socklens_v0.12.0_darwin_amd64.tar.gz"
      sha256 "f315cfbad82edda815f41dce8913fe040a27bfe2c0e651ec2c74f3b670d5eb0f"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.0/socklens_v0.12.0_darwin_arm64.tar.gz"
      sha256 "bd94a457a5e65686f4e9108906cece186f3181afe0584f15ec52858335a0608e"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.0/socklens_v0.12.0_linux_amd64.tar.gz"
      sha256 "ff34d093aa680ecdeebd4bb6e571a8dfdc939fb96677eadb423bef53f85a6ed7"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.0/socklens_v0.12.0_linux_arm64.tar.gz"
      sha256 "2b8b740960866b731d79ceaa76eafdb5b5caa93979857175095df48c046406bd"
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
