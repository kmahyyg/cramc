#!/bin/bash

GO_VER="1.24.3"
YARA_VER="4.3.2"
# build env
useradd -m user
echo "export GOPROXY=https://goproxy.cn,direct CGO_ENABLED=1" >> /etc/profile.d/golang.sh
echo "export GOROOT=/opt/golang" >> /etc/profile.d/golang.sh
echo 'export PATH=${GOROOT}/bin:${PATH}' >> /etc/profile.d/golang.sh
echo "export https_proxy=http://192.168.200.144:11086 http_proxy=http://192.168.200.144:11086 all_proxy=http://192.168.200.144:11086" >> ~/.bashrc
source /etc/profile.d/golang.sh
sed -i 's/main/main contrib non-free/g' ./sources.list
sed -i 's/deb.debian.org/mirrors.bfsu.edu.cn/g' ./sources.list
apt update -y
apt dist-upgrade -y
apt install gcc-mingw-w64-x86-64 build-essential nano vim pkg-config libyara-dev git zlib1g-dev libbz2-dev libmagic-dev autoconf libtool curl ca-certificates libjansson-dev flex bison libzstd-dev libssl-dev musl-tools upx sudo -y
cd /tmp
curl -L -O https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz
tar -zxvf go${GO_VER}.linux-amd64.tar.gz -C /opt
mv /opt/go /opt/golang
go install github.com/tc-hib/go-winres@latest
mkdir -p ~/softsrcs
curl -L -O https://github.com/VirusTotal/yara/archive/refs/tags/v${YARA_VER}.tar.gz
mv ./v${YARA_VER}.tar.gz ./yara-v${YARA_VER}.tar.gz
tar -xzvf yara-v${YARA_VER}.tar.gz -C ~/softsrcs
rm -rf /tmp/*.tar.gz
cd ~/softsrcs/yara-${YARA_VER}
./bootstrap.sh
# environment
PROJECT_DEST="/opt/buildtargets"
PROJECT_NAME="cramc_go"
export YARA_BUILD_LINUX_MUSL=${PROJECT_DEST}/yara/musl_linux_amd64
export YARA_SRC=/root/softsrcs/yara-${YARA_VER}
export PROJ_PREFIX_LINUX_MUSL=${PROJECT_DEST}/${PROJECT_NAME}/musl_linux_amd64
export YARA_BUILD_WIN64=${PROJECT_DEST}/yara/win64
export PROJ_PREFIX_WIN64=${PROJECT_DEST}/${PROJECT_NAME}/win64
mkdir -p ${PROJECT_DEST} ${YARA_BUILD_LINUX_MUSL} ${PROJ_PREFIX_LINUX_MUSL} ${YARA_BUILD_WIN64} ${PROJ_PREFIX_WIN64}
# For development with delve:  -gcflags "all=-N -l"
# cross-compile main program for win64
( cd ${YARA_BUILD_WIN64} && \
  ${YARA_SRC}/configure --host=x86_64-w64-mingw32 --prefix=${PROJ_PREFIX_WIN64} )
make -C ${YARA_BUILD_WIN64} install

GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  PKG_CONFIG_PATH=${PROJ_PREFIX_WIN64}/lib/pkgconfig \
      go build -trimpath \
      -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static\"" \
      -tags yara_static -o ../bin/cramc_scanner.exe ./cmd/scanonly/main.go

GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC=x86_64-w64-mingw32-gcc \
  PKG_CONFIG_PATH=${PROJ_PREFIX_WIN64}/lib/pkgconfig \
      go build -gcflags "all=-N -l" \
      -ldflags "-X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static\"" \
      -tags yara_static -o ../bin/cramc_dev_aio.exe ./cmd/aioagent/main.go
# static link in linux for devreleaser
( cd ${YARA_BUILD_LINUX_MUSL} && \
  ${YARA_SRC}/configure CC=musl-gcc --prefix=${PROJ_PREFIX_LINUX_MUSL} CFLAGS="-I/usr/include" CPPFLAGS="-I/usr/include")
make -C ${YARA_BUILD_LINUX_MUSL} install

GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
  CC=musl-gcc \
  PKG_CONFIG_PATH=${PROJ_PREFIX_LINUX_MUSL}/lib/pkgconfig \
      go build -trimpath \
      -ldflags "-s -w -X \"cramc_go/common.VersionStr=$(git describe --long --dirty --tags)\" -extldflags \"-static\"" \
      -tags yara_static -o ../bin/devreleaser ./cmd/devreleaser/main.go