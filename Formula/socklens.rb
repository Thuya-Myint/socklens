class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.6.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.0/socklens_v0.6.0_darwin_amd64.tar.gz"
      sha256 "b0d09456a30b5a9d457a6ba00874d18e99cb171ecf1215aa02fa5eb093ae84cd"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.0/socklens_v0.6.0_darwin_arm64.tar.gz"
      sha256 "ec1b19d4488cc695e7d91058073f0db9f0535f9029eae256f4ca7271c8cbc872"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.0/socklens_v0.6.0_linux_amd64.tar.gz"
      sha256 "a8dcad07bedb7bda4d3986965c546285b0c1888cf8c82439d277a8518d069892"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.6.0/socklens_v0.6.0_linux_arm64.tar.gz"
      sha256 "aef84a46ec50b2fcc505cd6aa2e96219e573cee9ec9d5c313d0f31bb7bcc02e4"
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
