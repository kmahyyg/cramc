package yarax_scanner

import (
	"bufio"
	"cramc_go/common"
	"cramc_go/customerrs"
	"os"
	"strings"
)

func ParseYaraScanResultText(inputfName string, outputChan chan *common.YaraScanResult) error {
	// this basically matches the default text output format for both yara 4.3.x+ and yara-x 1.x+
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
			common.Logger.Error("IGNORED - Current Line does NOT contain valid input: " + data)
			common.Logger.Error(customerrs.ErrInvalidInput.Error())
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
