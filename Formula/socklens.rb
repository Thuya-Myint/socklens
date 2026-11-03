class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.7.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.0/socklens_v0.7.0_darwin_amd64.tar.gz"
      sha256 "bebe0939082d360434e50edd4b2f3b2e3d62af88b8b3256a14b559aecbd479ad"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.0/socklens_v0.7.0_darwin_arm64.tar.gz"
      sha256 "ba9c38b2059876bdd37ba66beb492d48bc45e38d553da4b6f1bba8fca1234d9f"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.0/socklens_v0.7.0_linux_amd64.tar.gz"
      sha256 "036b84511f7679ea110639565c19e91ebc55ccc3aa36c488ca6ea337201a022d"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.0/socklens_v0.7.0_linux_arm64.tar.gz"
      sha256 "1134e501d141cca39458d3ada8d1a929ec09db1f245e86a9cc33de8276d779a0"
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
