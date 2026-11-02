class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.5.2"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.2/socklens_v0.5.2_darwin_amd64.tar.gz"
      sha256 "1905c8d1c0f6059d5154d3aa301c79d5f2f2abeb0ba31362173182f340b8d72b"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.2/socklens_v0.5.2_darwin_arm64.tar.gz"
      sha256 "7ef1fcdb22039ba0174b04c14d3ee58567c8d0b611547874b0510c4d4feef1fd"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.2/socklens_v0.5.2_linux_amd64.tar.gz"
      sha256 "465a7e97c7fc3f4ba6cfeb1561d68e18f7885e878c4398926c8b789992a22887"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.5.2/socklens_v0.5.2_linux_arm64.tar.gz"
      sha256 "67613a3c0e39664838f3f39d270bddddbebb3026fe0d4c76f99e56e64bf442c3"
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
