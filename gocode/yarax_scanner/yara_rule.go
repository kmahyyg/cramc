package yarax_scanner

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/customerrs"
	yarax "github.com/VirusTotal/yara-x/go"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func MergeAndCompile2UnifiedRules(plainTextRulesDir string, destUnifiedBinRulesPath string) {
	// DO NOT USE THIS FUNCTION IS MAIN AIOAGENT!
	yrCompiler, err := yarax.NewCompiler()
	if err != nil {
		panic(err)
	}
	// should NOT be necessary to call, unless memory leakage occurred, explicit GC
	// defer yrCompiler.Destroy()
	yrCompiler.NewNamespace("ycramc")
	common.Logger.Infoln("Yara Compiler Created.")
	// prepare buf
	rawRulesStrBuf := bytes.NewBuffer(nil)
	// iterate files
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
			// copy to buffer first
			_, err = io.Copy(rawRulesStrBuf, tmpFd)
			if err != nil {
				panic(err)
			}
			// add line break to make sure the format & syntax won't be broken
			// currently we don't support files that require additional metadata or external variables.
			_, _ = rawRulesStrBuf.WriteRune('\n')
			common.Logger.Infoln("Yara Compiler - File Added: ", curPath)
			return nil
		}
		return nil
	})
	// walk finished
	if err != nil {
		panic(err)
	}
	// let's go for compilation
	err = yrCompiler.AddSource(rawRulesStrBuf.String())
	if err != nil {
		panic(err)
	}
	common.Logger.Infoln("Yara Compiler - Rules Added To Pending List.")
	// now dump out
	compRules := yrCompiler.Build()
	// check compilation error
	compErrs := yrCompiler.Errors()
	if len(compErrs) > 0 {
		common.Logger.Errorln("Yara Compiler - Compilation Error: ", compErrs)
		panic(customerrs.ErrYaraXCompilationFailure)
	}
	common.Logger.Infoln("Yara Compiler - Rules Compiled.")
	// should NOT be necessary to call, explicit gc
	// defer compRules.Destroy()
	// open dumped out file fd
	destFd, err := os.Create(destUnifiedBinRulesPath)
	if err != nil {
		panic(err)
	}
	common.Logger.Infoln("Yara Compiler - Dumped File Created: ", destUnifiedBinRulesPath)
	defer destFd.Close()
	defer destFd.Sync()
	_, err = compRules.WriteTo(destFd)
	if err != nil {
		panic(err)
	}
	common.Logger.Infoln("Compiled Yara rules binary saved to: ", destUnifiedBinRulesPath)
}
