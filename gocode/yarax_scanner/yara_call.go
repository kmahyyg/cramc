package yarax_scanner

import (
	"archive/zip"
	"bytes"
	"cramc_go/common"
	"cramc_go/telemetry"
	yarax "github.com/VirusTotal/yara-x/go"
	"io"
	"os"
	"path"
)

// yarax_scanner.RecycleYaraResources() is deprecated, as yara-x official golang integration manages its allocated RAM automatically
// if any unexpected leakage happened, it would be better to create another struct holding all member resources and GC manually

func LoadRuleAndCreateYaraScanner(rules []byte) (*yarax.Scanner, error) {
	yrrRd := bytes.NewReader(rules)
	yrRules, err := yarax.ReadFrom(yrrRd)
	if err != nil {
		telemetry.CaptureException(err, "LoadRuleAndCreateYaraScanner")
		return nil, err
	}
	yrs := yarax.NewScanner(yrRules)
	return yrs, nil
}

func ScanFilesWithYara(yrr *yarax.Scanner, detList []string, outputChan chan *common.YaraScanResult) error {
	// had to decompress myself, so maintaining status data in memory is mandatory.
	// files pending for scan should only exist in `unknown_detection` key
	defer close(outputChan)
	for _, filep := range detList {
		fExt := path.Ext(filep)
		common.Logger.Info("Currently processing: " + filep)
		var mr *yarax.ScanResults
		if len(fExt) > 4 {
			// .xlsm,.xlsb
			vbaP, err := decompressMacroBin(filep)
			if err != nil {
				common.Logger.Error("Decompress Macro Failure: " + err.Error())
				continue
			}
			common.Logger.Info("Decompressed: " + filep)
			mr, err = yrr.Scan(vbaP)
			if err != nil {
				common.Logger.Error("Yara Scan Error:" + err.Error())
				continue
			}
		} else {
			// .xls, OLE
			// directly scan
			common.Logger.Info("Processing OLE Object File: " + filep)
			xlFile, err := os.ReadFile(filep)
			if err != nil {
				common.Logger.Error("Read OLE Object:" + err.Error())
				continue
			}
			mr, err = yrr.Scan(xlFile)
			if err != nil {
				common.Logger.Error("Scan Error: " + err.Error())
				continue
			}
		}
		for _, m := range mr.MatchingRules() {
			nDet := &common.YaraScanResult{
				DetectedRule: m.Identifier(),
				FilePath:     filep,
			}
			outputChan <- nDet
		}
		common.Logger.Info("Finished processing: " + filep)
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
		common.Logger.Info("Unable to find vbaProject.bin, ignore.")
		return nil, err
	}
	fBytes, err := io.ReadAll(vbaProjFile)
	return fBytes, err
}
