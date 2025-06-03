package yara_scanner

import (
	"bufio"
	"cramc_go/common"
	"cramc_go/customerrs"
	"os"
	"strings"
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

	for resLstScanner.Scan() {
		data := resLstScanner.Text()
		idx := strings.IndexRune(data, ' ')
		// if not found or overlap (at least two bytes after whitespace found)
		if idx <= 0 || idx+3 >= len(data) {
			common.Logger.Errorln("IGNORED - Current Line does NOT contain valid input: ", data)
			common.Logger.Errorln(customerrs.ErrInvalidInput)
			continue
		}
		_, ok := res[data[:idx]]
		if ok {
			res[data[:idx]] = append(res[data[:idx]], data[idx+1:])
		} else {
			res[data[:idx]] = []string{data[idx+1:]}
		}
	}
	if err = resLstScanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
