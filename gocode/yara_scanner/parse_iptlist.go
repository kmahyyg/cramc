package yara_scanner

import (
	"bufio"
	"os"
)

func ParseYaraScanResultText(inputfName string) (map[string][]string, error) {
	res := make(map[string][]string)
	resLstFd, err := os.OpenFile(inputfName, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer resLstFd.Close()
	resLstScanner := bufio.NewScanner(resLstFd)
	resLstScanner.Split(bufio.ScanLines)

}
