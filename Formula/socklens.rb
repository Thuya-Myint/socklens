class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.10.0"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.10.0/socklens_v0.10.0_darwin_amd64.tar.gz"
      sha256 "e22acf5d627e6abd28a140f178da1a513d8bb486ddb50dbc7adfbf8ff060e81f"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.10.0/socklens_v0.10.0_darwin_arm64.tar.gz"
      sha256 "35724195c13604e7184586d1ea935e5cf9f34cc0be52c8dc9d8cb2ba0b7b7983"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.10.0/socklens_v0.10.0_linux_amd64.tar.gz"
      sha256 "66930a3e171541777e2ab7258d244620eb1c4c55a50492c2be91541c9ef58f20"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.10.0/socklens_v0.10.0_linux_arm64.tar.gz"
      sha256 "0cb85d3a1443565f138b5196f7125f22ec24930f1bfa23fc0cffda5c6928c66c"
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
