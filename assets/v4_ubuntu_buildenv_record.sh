#!/bin/bash

export GO_VER="1.24.4"
export YARAX_VER="1.2.1"

apt update -y
apt install gcc-mingw-w64-x86-64 build-essential nano rustup vim pkg-config zip unzip libyara-dev git zlib1g-dev libbz2-dev libmagic-dev autoconf libtool curl ca-certificates libjansson-dev flex bison libzstd-dev libssl-dev musl-tools upx sudo libunwind-dev liblzma-dev -y

cd /tmp
curl -L -O https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz
tar -zxvf go${GO_VER}.linux-amd64.tar.gz -C /opt
mv /opt/go /opt/golang

export GOPROXY=https://goproxy.cn,direct CGO_ENABLED=1
export GOROOT=/opt/golang
export PATH="${GOROOT}/bin:${PATH}"
go install github.com/tc-hib/go-winres@latest

export PROJECT_DEST="/opt/buildtargets"
export PROJECT_NAME="cramc_go"
export THIRD_PARTY_SRC="/opt/softsrcs"
export YARAX_SRC=${THIRD_PARTY_SRC}/yara-x/yara-x-${YARAX_VER}
export PROJ_PREFIX_LINUX_GNU=${PROJECT_DEST}/${PROJECT_NAME}/linux_amd64
export PROJ_PREFIX_WIN_AMD64=${PROJECT_DEST}/${PROJECT_NAME}/win_amd64
mkdir -p ${THIRD_PARTY_SRC} ${PROJ_PREFIX_LINUX_GNU} ${PROJECT_DEST} ${PROJ_PREFIX_WIN_AMD64}

mkdir -p ${THIRD_PARTY_SRC}/yara-x
cd ${THIRD_PARTY_SRC}
curl -L -O https://github.com/VirusTotal/yara-x/archive/refs/tags/v${YARAX_VER}.tar.gz
mv ./v${YARAX_VER}.tar.gz ./yara-x-v${YARAX_VER}.tar.gz
tar -xzvf yara-x-v${YARAX_VER}.tar.gz -C ${THIRD_PARTY_SRC}/yara-x
rm -rf ./yara-x-v${YARAX_VER}.tar.gz
cd ${THIRD_PARTY_SRC}/yara-x/yara-x-${YARAX_VER}

rustup toolchain install 1.85.0
rustup default 1.85.0-x86_64-unknown-linux-gnu
rustup target add x86_64-unknown-linux-musl
rustup target add x86_64-pc-windows-gnu
cargo install cargo-c

# https://doc.rust-lang.org/rustc/codegen-options/index.html
# https://doc.rust-lang.org/nightly/rustc/platform-support.html
# export RUSTFLAGS="-C target-feature=+crt-static"
# library-type=staticlib must be provided, currently musl target does NOT support cdylib

# static link against glibc
export RUSTFLAGS="-C target-feature=+crt-static"
cargo cinstall -p yara-x-capi --release --crt-static --library-type staticlib --prefix ${PROJ_PREFIX_LINUX_GNU}

# or add dependencies: https://kennykerr.ca/rust-getting-started/understanding-windows-targets.html
# cargo add windows_x86_64_msvc@0.52.0 -p yara-x --target x86_64-pc-windows-gnu
# cargo add windows_x86_64_msvc@0.52.0 -p yara-x-capi --target x86_64-pc-windows-gnu

# clone my repo
# go build
GOOS=linux GOARCH=amd64 PKG_CONFIG_PATH="${PROJ_PREFIX_LINUX_GNU}/lib/x86_64-linux-gnu/pkgconfig" \
  go build -trimpath \
  -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static -lm -static-libgcc -static-libstdc++\"" \
  -tags static_link -o ../bin/devreleaser ./cmd/devreleaser

# for windows,  C API headers and static/dynamic libs are always included in each release
#cd /tmp
#curl -L -O https://github.com/VirusTotal/yara-x/releases/download/v1.2.1/yara-x-capi-v1.2.1-x86_64-pc-windows-msvc.zip
#mkdir -p ${PROJ_PREFIX_WIN_AMD64}/yara-x
#unzip -d ${PROJ_PREFIX_WIN_AMD64}/yara-x yara-x-capi-v1.2.1-x86_64-pc-windows-msvc.zip

# build c api for windows
cargo cinstall -p yara-x-capi --release --crt-static --library-type staticlib --target x86_64-pc-windows-gnu --prefix ${PROJ_PREFIX_WIN_AMD64}
# clone my repo
# hack for linker
cd "${PROJ_PREFIX_WIN_AMD64}/lib"
curl -L -O https://github.com/microsoft/windows-rs/raw/b62b802bae534fdaed3fa25b6838dc3001b6d084/crates/targets/x86_64_gnu/lib/libwindows.0.52.0.a

# go build
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc PKG_CONFIG_PATH="${PROJ_PREFIX_WIN_AMD64}/lib/pkgconfig" \
  go build -trimpath \
  -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static -lm -static-libgcc -static-libstdc++\"" \
  -tags static_link -o ../bin/cramc_aio.exe ./cmd/aioagent
