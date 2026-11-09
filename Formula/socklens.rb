class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.9.1"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.1/socklens_v0.9.1_darwin_amd64.tar.gz"
      sha256 "6c31197344f810a8cfa6ba8690a4695f28221b3a8e6560d212c52b1331b58963"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.1/socklens_v0.9.1_darwin_arm64.tar.gz"
      sha256 "3e89fc6a50ccd4ebe469b44ae831c071c41659e6610ac552b950a5737456cccf"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.1/socklens_v0.9.1_linux_amd64.tar.gz"
      sha256 "c6cc2150a6e12698a9f514b162c06fdc6329f85578935d233e1aae9d0bf89996"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.9.1/socklens_v0.9.1_linux_arm64.tar.gz"
      sha256 "290ce57eecb4123828d3b3b35f79c6230d1435bb14116c95195d5dfbe5ba6175"
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
