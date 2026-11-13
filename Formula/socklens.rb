class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.11.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.11.0/socklens_v0.11.0_darwin_amd64.tar.gz"
      sha256 "72d8823d3366c09b3636157b739cd2722ec5dd18cf6f8b27034be242a9283bae"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.11.0/socklens_v0.11.0_darwin_arm64.tar.gz"
      sha256 "317834ad894016ae300da71a9fed8ad5945a8fddb399becd76e786190f2c32ea"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.11.0/socklens_v0.11.0_linux_amd64.tar.gz"
      sha256 "77f9318bdf8545901c382253909f27262a3a172459e65f38c7f99af1f2246f11"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.11.0/socklens_v0.11.0_linux_arm64.tar.gz"
      sha256 "fe3b003fc4d8eb3315d3eedece3183fcba9b7796db6306e48bb585b69c8408b0"
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
