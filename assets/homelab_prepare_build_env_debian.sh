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
apt install gcc-mingw-w64-x86-64 build-essential nano vim pkg-config libyara-dev git zlib1g-dev libbz2-dev libmagic-dev autoconf libtool curl ca-certificates libjansson-dev flex bison libzstd-dev libssl-dev musl-tools sudo -y
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