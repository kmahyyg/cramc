package yarax_scanner

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/docparser"
	"cramc_go/fileutils"
	"cramc_go/platform/windoge_utils"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	yarax "github.com/VirusTotal/yara-x/go"
)

// yarax_scanner.RecycleYaraResources() is deprecated, as yara-x official golang integration manages its allocated RAM automatically
// if any unexpected leakage happened, it would be better to create another struct holding all member resources and GC manually

func LoadRuleAndCreateYaraScanner(rules []byte) (*yarax.Scanner, error) {
	yrrRd := bytes.NewReader(rules)
	yrRules, err := yarax.ReadFrom(yrrRd)
	if err != nil {
		return nil, err
	}
	yrs := yarax.NewScanner(yrRules)
	return yrs, nil
}

func SanitizeFilesWithYara(yrr *yarax.Scanner, inputChan chan string) error {
	// had to decompress myself, so maintaining status data in memory is mandatory.
	// files pending for scan should only exist in `unknown_detection` key
	var err error
	for filep := range inputChan {
		fExt := path.Ext(filep)
		common.Logger.Info("Currently processing: " + filep)
		cleanedPath := filepath.Clean(filep)
		// if on windows, try to retrieve file acl and owner
		// record original DACL
		var ownerSID_oriFile any
		var aclErrOccurred bool
		if common.IsRunningOnWin {
			if common.IsElevated || common.IsRunningBySYSTEM {
				ownerSID_oriFile, err = windoge_utils.RetrieveOwnerOfFile(cleanedPath)
				if err != nil {
					aclErrOccurred = true
					common.Logger.Error("Failed to retrieve owner SID for file: " + cleanedPath + ", error: " + err.Error())
				} else {
					common.Logger.Info("Owner SID retrieval completed for file: " + cleanedPath)
				}
			}
		}
		// start scan and sanitize
		var mr *yarax.ScanResults
		var vbas common.ExtractedVBAModules
		var legacyFlag bool
		if len(fExt) > 4 {
			var vbaP []byte
			// .xlsm,.xlsb
			legacyFlag = false
			vbaP, err = fileutils.DecompressMacroBin(filep)
			if err != nil {
				common.Logger.Error("Decompress Macro Failure: " + err.Error())
				continue
			}
			common.Logger.Info("Decompressed: " + filep)
			vbas, err = docparser.ExtractVBACode(vbaP, legacyFlag)
			if err != nil {
				common.Logger.Error("Extract VBACode Error: " + err.Error() + ", File: " + filep)
				continue
			}
		} else {
			// .xls, OLE
			legacyFlag = true
			// directly scan
			common.Logger.Info("Processing OLE Object File: " + filep)
			var xlFile []byte
			xlFile, err = os.ReadFile(filep)
			if err != nil {
				common.Logger.Error("Read OLE Object Error:" + err.Error())
				continue
			}
			vbas, err = docparser.ExtractVBACode(xlFile, legacyFlag)
			if err != nil {
				common.Logger.Error("Extract VBACode Error: " + err.Error() + ", File: " + filep)
				continue
			}
		}
		var detectedListLock = &sync.Mutex{}
		var detectedList []*common.YaraScanResult
		for _, mod := range vbas {
			mr, err = yrr.Scan(mod.SourceCode)
			if err != nil {
				common.Logger.Error("Scan Error: " + err.Error())
				continue
			}
			// redundant data structure for further extension
			for _, m := range mr.MatchingRules() {
				nDet := &common.YaraScanResult{
					DetectedRule: m.Identifier(),
					FilePath:     filep,
					Module:       mod.ModuleName,
				}
				common.Logger.Info("Scan Found: " + m.Identifier() + " in Module: " + mod.ModuleName)
				detectedListLock.Lock()
				detectedList = append(detectedList, nDet)
				detectedListLock.Unlock()
			}
		}
		// send to replace-stream for modification
		if len(detectedList) != 0 {
			common.Logger.Info("Found " + strconv.Itoa(len(detectedList)) + " Yara matches in " + filep)
			// start backup first
			err = fileutils.ZstdBakFile(filep)
			if err != nil {
				common.Logger.Error("Action Aborted, Backup failed for: " + filep)
				continue
			}
			common.Logger.Info("Backup completed for: " + filep)
			// start sanitize
			var infectedModulesLock = &sync.Mutex{}
			var infectedModules []string
			for _, d := range detectedList {
				infectedModulesLock.Lock()
				infectedModules = append(infectedModules, d.Module)
				infectedModulesLock.Unlock()
			}
			infectedModules = slices.Compact(infectedModules)
			err = docparser.ReplaceMaliciousCode(filep, infectedModules, legacyFlag)
			if err != nil {
				common.Logger.Error("Error replacing malicious code: " + err.Error())
				continue
			}
		}
		// finally replace temp file to original one
		err = os.Rename(filep+".tmp", filep)
		if err != nil {
			common.Logger.Error("Aborted, Renaming temporary file error: " + err.Error())
			continue
		}
		common.Logger.Info("Malicious code successfully removed from file: " + filep)
		// fix file owner and acl if elevated
		if common.IsRunningOnWin {
			if common.IsElevated || common.IsRunningBySYSTEM {
				if aclErrOccurred {
					common.Logger.Info("Skip fixing permissions for file: " + filep + " due to ACL retrieval error.")
					continue
				} else {
					// try fix file acl
					common.Logger.Info("Detected running elevated. Start Fixing.")
					if ownerSID_oriFile != nil {
						err = windoge_utils.SetOwnerOfFile(cleanedPath, ownerSID_oriFile)
						if err != nil {
							common.Logger.Error("Failed to set owner for file: " + cleanedPath + ", error: " + err.Error())
							continue
						}
						common.Logger.Info("Owner fix completed for file: " + cleanedPath)
					}
				}
			} else {
				common.Logger.Info("It's not necessary to fix permissions for non-elevated user.")
			}
		}
		common.Logger.Info("Finished remediating and fixing permissions of: " + filep)
	}
	return nil
}
