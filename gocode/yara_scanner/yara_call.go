package yara_scanner

import (
	"archive/zip"
	"bytes"
	"cramc_go/common"
	"github.com/hillu/go-yara/v4"
	"io"
	"os"
	"path"
)

func RecycleYaraResources() {
	_ = yara.Finalize()
}

func LoadRuleAndCreateYaraScanner(rules []byte) (*yara.Scanner, error) {
	yrrRd := bytes.NewReader(rules)
	yrr, err := yara.ReadRules(yrrRd)
	if err != nil {
		return nil, err
	}
	yrs, err := yara.NewScanner(yrr)
	if err != nil {
		return nil, err
	}
	return yrs, nil
}

func ScanFilesWithYara(yrr *yara.Scanner, detList []string, outputChan chan *common.YaraScanResult) error {
	// had to decompress myself, so maintaining status data in memory is mandatory.
	// files pending for scan should only exist in `unknown_detection` key
	defer close(outputChan)
	for _, filep := range detList {
		fExt := path.Ext(filep)
		common.Logger.Infoln("Currently processing: ", filep)
		var mr yara.MatchRules
		if len(fExt) > 4 {
			// .xlsm,.xlsb
			vbaP, err := decompressMacroBin(filep)
			if err != nil {
				common.Logger.Errorln(err)
				continue
			}
			common.Logger.Infoln("Decompressed: ", filep)
			err = yrr.SetCallback(&mr).ScanMem(vbaP)
			if err != nil {
				common.Logger.Errorln(err)
				continue
			}
		} else {
			// .xls, OLE
			// directly scan
			common.Logger.Infoln("Processing OLE Object File: ", filep)
			xlFile, err := os.ReadFile(filep)
			if err != nil {
				common.Logger.Errorln(err)
				continue
			}
			err = yrr.SetCallback(&mr).ScanMem(xlFile)
			if err != nil {
				common.Logger.Errorln(err)
				continue
			}
		}
		for _, m := range mr {
			nDet := &common.YaraScanResult{
				DetectedRule: m.Rule,
				FilePath:     filep,
			}
			outputChan <- nDet
		}
		common.Logger.Infoln("Finished processing: ", filep)
	}
	return nil
}

func decompressMacroBin(fPath string) ([]byte, error) {
	zRd, err := zip.OpenReader(fPath)
	if err != nil {
		return nil, err
	}
	defer zRd.Close()
	vbaProjFile, err := zRd.Open("xl/vbaProject.bin")
	if err != nil {
		common.Logger.Infoln("Unable to find vbaProject.bin, ignore.")
		return nil, err
	}
	fBytes, err := io.ReadAll(vbaProjFile)
	return fBytes, err
}
