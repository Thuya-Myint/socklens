class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.2.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.2.0/socklens_v0.2.0_darwin_amd64.tar.gz"
      sha256 "da3a9f390ebf921cf0474c02ae76603c5215a124a0b77f51047b677adfd267d4"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.2.0/socklens_v0.2.0_darwin_arm64.tar.gz"
      sha256 "a26253d73e26b5483acc175b9e90aac580107885fd06178bbd804db0f8940fcd"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.2.0/socklens_v0.2.0_linux_amd64.tar.gz"
      sha256 "59e7580b42ab919d02a32a38f3b48c1a2270e556c05d2b014ac0cfbb22596945"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.2.0/socklens_v0.2.0_linux_arm64.tar.gz"
      sha256 "bff82bb16cd34c7ce4070c72af691411b618b1a3ceee7def99c69d32dc35f9d1"
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
