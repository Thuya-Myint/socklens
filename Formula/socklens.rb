class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.5.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.0/socklens_v0.5.0_darwin_amd64.tar.gz"
      sha256 "c9260fcb54ac9a3ef9903a801b60a70b1f1b21bc8535d1def79417d4352e16f4"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.0/socklens_v0.5.0_darwin_arm64.tar.gz"
      sha256 "9add1d111419b61591dd0fee6196bde6d5b1f4c18a59e17053fd9cff3f155529"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.0/socklens_v0.5.0_linux_amd64.tar.gz"
      sha256 "f4de1f3dfba5f840799c80914a4bae7ec38969e8a2c6f275a2b83e9cc8056881"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.0/socklens_v0.5.0_linux_arm64.tar.gz"
      sha256 "f15d41aa3bfcdcd3cb86e7205902465ecbf8b9ca15d711e497e507e1e3f9245f"
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
