class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.7.2"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.2/socklens_v0.7.2_darwin_amd64.tar.gz"
      sha256 "ea4023843a16011fa76aeed921ba2187ad13cc25ef004f344759fbdbd028a628"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.2/socklens_v0.7.2_darwin_arm64.tar.gz"
      sha256 "3bc88028772627a15de0d90096aab58b80bf107955aa6c82906343785c79fbc9"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.2/socklens_v0.7.2_linux_amd64.tar.gz"
      sha256 "c24a7ec3ee12b8d5d8e196ab6b33c80e953f98d2e9368e3b4a540462cd7e98e5"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.2/socklens_v0.7.2_linux_arm64.tar.gz"
      sha256 "59c02cb9016ba4f0603b1e8498077160d574695ff16e8042c3fcc694ef20b711"
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
