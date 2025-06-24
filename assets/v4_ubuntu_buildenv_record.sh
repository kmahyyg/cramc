#!/bin/bash

export GO_VER="1.24.4"
export YARAX_VER="1.2.1"

apt update -y
apt install gcc-mingw-w64-x86-64 build-essential nano rustup vim pkg-config libyara-dev git zlib1g-dev libbz2-dev libmagic-dev autoconf libtool curl ca-certificates libjansson-dev flex bison libzstd-dev libssl-dev musl-tools upx sudo libunwind-dev liblzma-dev -y

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
export PROJ_PREFIX_LINUX_MUSL=${PROJECT_DEST}/${PROJECT_NAME}/musl_linux_amd64
export YARAX_BUILD_LINUX_MUSL=${PROJECT_DEST}/yara-x/musl_linux_amd64
mkdir -p ${THIRD_PARTY_SRC} ${PROJ_PREFIX_LINUX_MUSL} ${YARAX_BUILD_LINUX_MUSL} ${PROJECT_DEST}

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
cargo install cargo-c

# https://doc.rust-lang.org/rustc/codegen-options/index.html
# https://doc.rust-lang.org/nightly/rustc/platform-support.html
# export RUSTFLAGS="-C target-feature=+crt-static"
# library-type=staticlib must be provided, currently musl target does NOT support cdylib

# static link against glibc
export RUSTFLAGS="-C target-feature=+crt-static"
cargo cinstall -p yara-x-capi --release --crt-static --library-type staticlib --prefix /target/build/proj

# clone my repo
PKG_CONFIG_PATH="/target/build/proj/lib/x86_64-linux-gnu/pkgconfig" go build -trimpath -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static -lm -static-libgcc -static-libstdc++\""  -tags static_link -o ../bin/devreleaser ./cmd/devreleaser

# for windows,  C API headers and static/dynamic libs are always included in each release

