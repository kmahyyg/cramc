package yara_scanner

import (
	"bufio"
	"cramc_go/common"
	"cramc_go/customerrs"
	"os"
	"strings"
)

func ParseYaraScanResultText(inputfName string, outputChan chan *common.YaraScanResult) error {
	defer close(outputChan)
	resLstFd, err := os.OpenFile(inputfName, os.O_RDONLY, 0644)
	if err != nil {
		return err
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
		nYRResult := &common.YaraScanResult{
			DetectedRule: data[:idx],
			FilePath:     data[idx+1:],
		}
		outputChan <- nYRResult
	}
	if err = resLstScanner.Err(); err != nil {
		return err
	}
	return nil
}
