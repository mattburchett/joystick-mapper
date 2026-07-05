# Maintainer: Matt Burchett
pkgname=joystick-mapper
pkgver=0.1.0
pkgrel=1
pkgdesc="Map joystick buttons and axes to programmable keyboard sequences, with per-game profiles and a tray icon"
arch=('x86_64')
url="https://github.com/mattburchett/joystick-mapper"
license=('MIT')
depends=('glibc' 'libgl' 'libx11' 'libxcursor' 'libxi' 'libxinerama' 'libxrandr' 'libxxf86vm')
makedepends=('go' 'gcc')
source=("$pkgname-$pkgver.tar.gz::$url/archive/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
  cd "$pkgname-$pkgver"
  export CGO_ENABLED=1
  export CGO_CPPFLAGS="${CPPFLAGS}"
  export CGO_CFLAGS="${CFLAGS}"
  export CGO_CXXFLAGS="${CXXFLAGS}"
  export CGO_LDFLAGS="${LDFLAGS}"
  export GOFLAGS="-buildmode=pie -trimpath -mod=vendor -modcacherw"
  go build -ldflags "-linkmode=external -s -w" -o "$pkgname" .
}

package() {
  cd "$pkgname-$pkgver"
  install -Dm755 "$pkgname" "$pkgdir/usr/bin/$pkgname"
  install -Dm644 assets/joystick-mapper.desktop "$pkgdir/usr/share/applications/$pkgname.desktop"
  install -Dm644 assets/icon.png "$pkgdir/usr/share/icons/hicolor/128x128/apps/$pkgname.png"
  install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}
