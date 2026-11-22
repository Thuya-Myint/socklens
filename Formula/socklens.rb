class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.12.1"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.1/socklens_v0.12.1_darwin_amd64.tar.gz"
      sha256 "bba6d669ff74944d2a136dccc52cc174e1b500178da0940c2ae5deaf247d38ac"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.1/socklens_v0.12.1_darwin_arm64.tar.gz"
      sha256 "84b0323bbc4524d9accff07e994588bcbdf5015dc95cf8cd512bd0ccdda500c0"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.1/socklens_v0.12.1_linux_amd64.tar.gz"
      sha256 "e02eab00397d9267288e6d59a0575bffcbfd84038252a3cc040272ccb0157840"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.12.1/socklens_v0.12.1_linux_arm64.tar.gz"
      sha256 "cb0fd055562a13563ecc855fedcd2a0621b3a9306dc2abe7561a8093e1204d65"
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
