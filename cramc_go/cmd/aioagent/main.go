//go:generate go-winres make --product-version=git-tag --file-version=git-tag

package main

import (
	"context"
	"cramc_go/common"
	"cramc_go/cryptutils"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"cramc_go/platform/windoge_utils"
	"cramc_go/updchecker"
	"cramc_go/yarax_scanner"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

const (
	yaraRulesPath = "unified.yar.bin"
)

var (
	flActionPath = flag.String("actionPath", "C:\\", "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User Profile folder only. By default, we use recursive search on whole disk.")
	flDryRun     = flag.Bool("dryRun", false, "Scan only, take no action on files, record action to be taken in log.")
	allowedExts  = []string{".xls", ".xlsx", ".xlsm", ".xlsb"}
	flHelp       = flag.Bool("help", false, "Show help")
	flSkipUpdChk = flag.Bool("skipUpdChk", false, "Development only: set to true to skip update checker.")
)

func init() {
	flag.Parse()
}

func main() {
	if *flHelp {
		flag.PrintDefaults()
		return
	}
	// init logging
	logger, logfd := logging.NewLogger("cramc_go.log")
	common.Logger = logger
	defer logfd.Close()
	defer logfd.Sync()

	// startup behavior
	common.Logger.Info("Welcome to CRAMC!")
	common.Logger.Info("Current Version: " + common.VersionStr)

	// panic capture
	defer func() {
		if r := recover(); r != nil {
			debug.PrintStack()
			common.Logger.Log(context.TODO(), logging.LevelFatal, fmt.Sprintf("Panic on [%v], check stacktrace: %s.", r, string(debug.Stack())))
			os.Exit(1)
		}
	}()

	// startup check
	if finfo, err := os.Stat(*flActionPath); err != nil || !finfo.IsDir() {
		common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrActionPathMustBeDir.Error())
		os.Exit(-1)
	}
	common.Logger.Info("Initial args-check passed.")
	// read and decrypt config file
	hPwdBytes, err := hex.DecodeString(common.HexEncryptionPassword)
	if err != nil {
		common.Logger.Info("Cannot prepare password.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	// fix #9, unable to find database or yara rules
	execPath, err := os.Executable()
	if err != nil {
		common.Logger.Info("Cannot get executable path.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	execDir := filepath.Dir(execPath)
	// dry run is always handled by callee to make sure behavior consistent.
	common.DryRunOnly = *flDryRun
	// kill M365 office processes on windows
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Info("Triggered M365 Office processes killer.")
	// update checker
	latestV, err := updchecker.CheckUpdateFromInternet()
	common.Logger.Info("Called update-checker.")
	if err != nil {
		common.Logger.Error("Update Checker Error: " + err.Error())
	} else {
		if *flSkipUpdChk {
			common.Logger.Info("UpdateChecker skipped due to flag set.")
		} else {
			if latestV.ProgramRevision != common.ProgramRev {
				common.Logger.Log(context.TODO(), logging.LevelFatal, "Program UpdCheck: "+customerrs.ErrNotLatestVersion.Error())
				os.Exit(-1)
			}
		}
	}
	// check privilege
	isElevated, _ := windoge_utils.CheckProcessElevated()
	isRunningBySYSTEM, _ := windoge_utils.CheckRunningBySYSTEM()
	common.IsElevated = isElevated
	common.IsRunningBySYSTEM = isRunningBySYSTEM
	common.Logger.Info("Privilege and platform check passed.")
	// if noDiskScan set, directly go for yara scanner
	var wg = &sync.WaitGroup{}

	// --------- KEY SECTION: ASYNC PROCESS - MAIN PROCEDURE ---------
	// SECTION 1: SEARCHER, SCAN INPUT PRODUCER
	// searcher output channel
	var searcherOptChan = make(chan string, 50)
	var scanIptChan = make(chan string, 50)
	// searcher result consumer and scanner producer
	var searchConsumer = func() {
		defer func() {
			// close channel on producer
			close(scanIptChan)
			common.Logger.Debug("Scan Input Channel Closed in SearchConsumer.")
		}()
		// search result process
		for item := range searcherOptChan {
			common.Logger.Info("Found file: " + item)
			// check on disk size
			fExistsOnDisk, fSize, _ := fileutils.CheckFileOnDiskSize(item)
			if !fExistsOnDisk || fSize <= 0 {
				common.Logger.Info("File Not On Local Disk, Ignore: " + item)
				continue
			} else {
				// create producer for sending to yara scanner
				scanIptChan <- item
				common.Logger.Info("Send file for parsing and yara scan: " + item)
			}
		}
		common.Logger.Info("searchConsumer finished.")
	}
	// searcher procedure
	// start search consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		searchConsumer()
	}()
	// init general searcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		common.Logger.Info("GenrealWalkthroughSearcher started.")
		counted, err := fileutils.GeneralWalkthroughSearch(*flActionPath, allowedExts, searcherOptChan)
		// should not encounter some unexpected error
		if err != nil {
			common.Logger.Error("Unwanted error in GeneralSearcher: " + err.Error())
			common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrUnknownInternalError.Error())
			os.Exit(1)
		}
		common.Logger.Info(fmt.Sprintf("Found %d File using GeneralSearcher, proceed to next step.", counted))
		return
	}()
	// SECTION 2: YARA-X SCANNER, SCAN RESULTS PRODUCER, ALSO SANITIZE ON THE FLY
	// searcher finished, go for yara scanner
	// read yara rules and decrypt
	// fix #9
	yaraRulesAbsPath := filepath.Join(execDir, yaraRulesPath)
	common.Logger.Debug("DEBUG: Yara Rules path: " + yaraRulesAbsPath)
	yrRulesEncBin, err := os.ReadFile(yaraRulesAbsPath)
	if err != nil {
		common.Logger.Info("Could not read yara compiled rules file.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	common.Logger.Info("Compiled yara rules read.")
	_, yrRuleBin, err := cryptutils.XChacha20Decrypt(hPwdBytes, yrRulesEncBin)
	if err != nil {
		common.Logger.Info("Could not decrypt yara compiled rules file.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	// build scanner instance
	yrScanner, err := yarax_scanner.LoadRuleAndCreateYaraScanner(yrRuleBin)
	if err != nil {
		common.Logger.Info("Unable to create yara scanner with provided rule.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	// no need to call yaraX_Scanner.destroy() as GC will handle, this is documented in godoc.
	common.Logger.Info("Yara scanner loaded successfully.")
	// producer set
	// go to scan against rules
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = yarax_scanner.SanitizeFilesWithYara(yrScanner, scanIptChan)
		if err != nil {
			common.Logger.Error("Yara scanner returned err when exit: " + err.Error())
		}
		common.Logger.Info("Yara scanner finished.")
	}()
	// wait for all procedures
	wg.Wait()
	// wait for 5 seconds for cleanup
	runtime.GC()
	time.Sleep(3 * time.Second)
	common.Logger.Info("All done. Program exited.")
}
