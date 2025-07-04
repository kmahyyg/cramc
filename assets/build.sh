#!/bin/bash

set -x
set -e

# arguments check

if [ $# -lt 2 ]; then
    echo "Usage: $0 <target_platform> <target_arch>"
    exit 1
fi

# shared vars

export YARAX_SRC=${GITHUB_WORKSPACE}/yara-x
mkdir -p ${THIRD_PARTY_SRC} ${PROJECT_DEST}

# install deps

sudo apt update -y
sudo apt install gcc-mingw-w64-x86-64 build-essential pkg-config zip unzip git zlib1g-dev libbz2-dev libmagic-dev \
     autoconf libtool curl ca-certificates libjansson-dev flex bison libzstd-dev libssl-dev upx \
     libunwind-dev liblzma-dev tree -y
go install github.com/tc-hib/go-winres@latest
# Dev Dependencies: brew install protobuf
# Dev Dependencies: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# prepare golang code
cd ${GITHUB_WORKSPACE}/cramc/gocode
mkdir -p ../bin

# download yara-x
#
# already handled by CI (Github Action)
#
# mkdir -p ${THIRD_PARTY_SRC}/yara-x
# cd ${THIRD_PARTY_SRC}
# curl -L -O https://github.com/VirusTotal/yara-x/archive/refs/tags/v${YARAX_VER}.tar.gz
# mv ./v${YARAX_VER}.tar.gz ./yara-x-v${YARAX_VER}.tar.gz
# tar -xzvf yara-x-v${YARAX_VER}.tar.gz -C ${THIRD_PARTY_SRC}/yara-x
# rm -rf ./yara-x-v${YARAX_VER}.tar.gz
# cd ${THIRD_PARTY_SRC}/yara-x/yara-x-${YARAX_VER}

# install cargo-c
#
# $ cargo install cargo-c
# already handled by CI (GitHub Action)

# set build env for yara-x
#
# ref comment:
# https://doc.rust-lang.org/rustc/codegen-options/index.html
# https://github.com/VirusTotal/yara-x/issues/181
# https://github.com/VirusTotal/yara-x/issues/185 Damn SLOW when against glibc.
# GNU GLIBC can be forced to be statically linked
export RUSTFLAGS="-C target-feature=+crt-static"

# start build
if [[ "$1" == "linux" ]]; then
    export PROJ_PREFIX_LINUX_GNU=${PROJECT_DEST}/${PROJECT_NAME}/linux_amd64
    mkdir -p "${PROJ_PREFIX_LINUX_GNU}"
    # build yara-x for linux
    cd "${YARAX_SRC}"
    cargo cinstall -p yara-x-capi --release --crt-static --library-type staticlib --prefix=${PROJ_PREFIX_LINUX_GNU}
    # build golang code - devreleaser
    cd ${GITHUB_WORKSPACE}/cramc/gocode

    if [[ "$GITHUB_REF_TYPE" == "tag" ]]; then
        GOOS=linux GOARCH=amd64 CGO_ENABLED=1 PKG_CONFIG_PATH=${PROJ_PREFIX_LINUX_GNU}/lib/x86_64-linux-gnu/pkgconfig \
        go build -trimpath -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static -lm -static-libgcc -static-libstdc++\"" -tags static_link -o ../bin/devreleaser ./cmd/devreleaser
        upx -9 ../bin/devreleaser
    elif [[ "$GITHUB_REF_TYPE" == "branch" ]]; then
        GOOS=linux GOARCH=amd64 CGO_ENABLED=1 PKG_CONFIG_PATH=${PROJ_PREFIX_LINUX_GNU}/lib/x86_64-linux-gnu/pkgconfig \
        go build -ldflags "-X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static -lm -static-libgcc -static-libstdc++\"" \
        -tags static_link -gcflags 'all=-N -l' -o ../bin/devreleaser ./cmd/devreleaser
    fi

    # build yara rules and cleanup database
    cp -ar ${GITHUB_WORKSPACE}/cramc/bin/devreleaser ${GITHUB_WORKSPACE}/cramc/assets
    cd ${GITHUB_WORKSPACE}/cramc/assets
    ./devreleaser -compile
    ./devreleaser -enc=true -in=./yrules/bin/unified.yar -out=../bin/unified.yar.bin
    ./devreleaser -enc=true -in=./cramc_db.json -out=../bin/cramc_db.bin
    # cleanup dev releaser
    rm -f ./devreleaser
    # prepare for uploading artifacts
    cd ${GITHUB_WORKSPACE}/cramc/bin
    mv ./devreleaser ./devreleaser_linux_amd64
    # check results for debug
    ls -alh ${GITHUB_WORKSPACE}/cramc/bin
elif [[ "$1" == "windows" ]]; then
    export PROJ_PREFIX_WIN_AMD64=${PROJECT_DEST}/${PROJECT_NAME}/win_amd64
    mkdir -p "${PROJ_PREFIX_WIN_AMD64}"
    # build yara-x for windows
    cd "${YARAX_SRC}"
    cargo cinstall -p yara-x-capi --release --crt-static --library-type staticlib --target x86_64-pc-windows-gnu --prefix=${PROJ_PREFIX_WIN_AMD64}
    # workaround for linker (windows-rs 0.52.0)
    cd "${PROJ_PREFIX_WIN_AMD64}/lib"
    cp -ar "${GITHUB_WORKSPACE}/cramc/assets/linkerdeps/lib/libwindows.0.52.0.a" .
    # curl -L -O https://github.com/microsoft/windows-rs/raw/b62b802bae534fdaed3fa25b6838dc3001b6d084/crates/targets/x86_64_gnu/lib/libwindows.0.52.0.a
    # generate exe winres
    cd ${GITHUB_WORKSPACE}/cramc/gocode/cmd/aioagent
    GOOS=windows GOARCH=amd64 go-winres make --product-version=git-tag --file-version=git-tag
    # build golang code - aioagent & privhelper rpc server
    cd ${GITHUB_WORKSPACE}/cramc/gocode

    if [[ "$GITHUB_REF_TYPE" == "tag" ]]; then
        GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
        PKG_CONFIG_PATH=${PROJ_PREFIX_WIN_AMD64}/lib/pkgconfig CC=x86_64-w64-mingw32-gcc \
        go build -trimpath -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" \
        -extldflags \"-static -lm -static-libgcc -static-libstdc++\"" -tags static_link -o ../bin/cramc_aio.exe ./cmd/aioagent

        GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\"" -o ../bin/privhelper.exe ./cmd/privhelper

        # only compress in prod release
        upx -9 ../bin/cramc_aio.exe
        upx -9 ../bin/privhelper.exe
    elif [[ "$GITHUB_REF_TYPE" == "branch" ]]; then
        GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
        PKG_CONFIG_PATH=${PROJ_PREFIX_WIN_AMD64}/lib/pkgconfig CC=x86_64-w64-mingw32-gcc \
        go build -gcflags 'all=-N -l' -ldflags "-X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" \
        -extldflags \"-static -lm -static-libgcc -static-libstdc++\"" -tags static_link -o ../bin/cramc_aio.exe ./cmd/aioagent

        GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -gcflags 'all=-N -l' \
        -ldflags "-X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\"" -o ../bin/privhelper.exe ./cmd/privhelper
    fi

    # check results for debug
    ls -alh ${GITHUB_WORKSPACE}/cramc/bin
else
    echo "Unsupported OS."
    exit 1
fi

exit 0
