package yara_scanner

import (
	"cramc_go/common"
	"github.com/hillu/go-yara/v4"
	"os"
	"path/filepath"
	"strings"
)

func MergeAndCompile2UnifiedRules(plainTextRulesDir string, destUnifiedBinRulesPath string) {
	// DO NOT USE THIS FUNCTION IS MAIN AIOAGENT!
	// THIS WILL KILL ALL YARA-RELATED ALLOCATED RESOURCES IN PROGRAM!
	yrCompiler, err := yara.NewCompiler()
	if err != nil {
		panic(err)
	}
	common.Logger.Infoln("Yara Compiler Created.")
	defer yara.Finalize()
	// should NOT be necessary to call, unless memory leakage occurred.
	// defer yrCompiler.Destroy()
	err = filepath.Walk(plainTextRulesDir, func(curPath string, curInfo os.FileInfo, err error) error {
		if err != nil {
			common.Logger.Errorln("Iterating Files err: ", err)
		}
		if curInfo.IsDir() {
			return nil
		}
		if strings.HasSuffix(curInfo.Name(), ".yar") {
			tmpFd, err := os.OpenFile(curPath, os.O_RDONLY, 0644)
			if err != nil {
				return err
			}
			defer tmpFd.Close()
			err = yrCompiler.AddFile(tmpFd, "ycramc")
			if err != nil {
				panic(err)
			}
			common.Logger.Infoln("Yara Compiler - File Added: ", curPath)
			return nil
		}
		return nil
	})
	compRules, err := yrCompiler.GetRules()
	if err != nil {
		panic(err)
	}
	common.Logger.Infoln("Yara Compiler - Rules Compiled.")
	err = compRules.Save(destUnifiedBinRulesPath)
	if err != nil {
		panic(err)
	}
	common.Logger.Infoln("Compiled Yara rules binary saved to: ", destUnifiedBinRulesPath)
}
