class Socklens < Formula
  desc "Inspect and monitor local ports"
  homepage "https://github.com/Thuya-Myint/socklens"
  version "0.7.1"
  license "MIT"
  on_macos do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.1/socklens_v0.7.1_darwin_amd64.tar.gz"
      sha256 "1f8b10b4e573eddaafaa7807e11374fd1567bef5e8fc11bb8e68b42de23ee27e"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.1/socklens_v0.7.1_darwin_arm64.tar.gz"
      sha256 "e43a3fa9bfe7652aac6428ac72de2b3a7df62d6d086b6408e4e64fdf83003c40"
    end
  end
  on_linux do
    on_intel do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.1/socklens_v0.7.1_linux_amd64.tar.gz"
      sha256 "23dbbcfc64f6ea8958197a344532865c94504d5f48da121709a3ad955189b3d3"
    end
    on_arm do
      url "https://github.com/Thuya-Myint/socklens/releases/download/v0.7.1/socklens_v0.7.1_linux_arm64.tar.gz"
      sha256 "7aec795d860194d5655bb8321c670b3e32e7fd0af5d6ecc130897e591fa6c686"
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
