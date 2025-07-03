//go:generate go-winres make --product-version=git-tag

package main

import (
	"context"
	"cramc_go/common"
	"cramc_go/cryptutils"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/hardener"
	"cramc_go/logging"
	"cramc_go/platform/windoge_utils"
	"cramc_go/sanitizer_ole"
	"cramc_go/telemetry"
	"cramc_go/updchecker"
	"cramc_go/yarax_scanner"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	databasePath  = "cramc_db.bin"
	yaraRulesPath = "unified.yar.bin"
	iptFileList   = "ipt_yrscan.lst"

	betterStackURL         = "https://s1358347.eu-nbg-2.betterstackdata.com"
	betterStackBearerToken = "26Y9ahkqDMsQgLN9yTb1JETU"
)

var (
	flActionPath      = flag.String("actionPath", "C:\\Users", "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. By default, we use recursive search.")
	flDryRun          = flag.Bool("dryRun", false, "Scan only, take no action on files, record action to be taken in log.")
	flEnableHardening = flag.Bool("enableHardening", true, "Enables hardening measure to prevent further infection. Windows OS only.")
	flNoDiskScan      = flag.Bool("noDiskScan", false, "Do not scan files on disk, but supply file list. If platform is not Windows x86_64, yara won't work, you have to set this to true and then run Yara scanner against our rules and save output to ipt_yrscan.lst. Yara-X scanner is not supported yet.")
	allowedExts       = []string{".xls", ".xlsx", ".xlsm", ".xlsb"}
	flHelp            = flag.Bool("help", false, "Show help")
	flNoPriv          = flag.Bool("noPriv", false, "Do not run as privileged user, even you are privileged.")
	flSkipUpdChk      = flag.Bool("skipUpdChk", false, "Development only: set to true to skip update checker.")
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

	// init telemetry
	telemetry.Init(common.VersionStr)
	bsSender := telemetry.NewBetterStackSender(betterStackURL, betterStackBearerToken)
	bsSender.SetDefaultSender()

	// startup behavior
	common.Logger.Info("Welcome to CRAMC!")
	common.Logger.Info("Current Version: " + common.VersionStr)
	if finfo, err := os.Stat(*flActionPath); err != nil || !finfo.IsDir() {
		common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrActionPathMustBeDir.Error())
		os.Exit(-1)
	}
	if *flNoDiskScan {
		iptFinfo, err := os.Stat(iptFileList)
		if err != nil || iptFinfo.IsDir() {
			common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrNoScanSetButNoListProvided.Error())
			os.Exit(-1)
		}
	}
	common.Logger.Info("Initial args-check passed.")
	// read and decrypt config file
	hPwdBytes, err := hex.DecodeString(common.HexEncryptionPassword)
	if err != nil {
		common.Logger.Info("Cannot prepare password.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	// fix #9
	execPath, err := os.Executable()
	if err != nil {
		common.Logger.Info("Cannot get executable path.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	execDir := filepath.Dir(execPath)
	databaseAbsPath := filepath.Join(execDir, databasePath)
	common.Logger.Debug("DEBUG: Database path: " + databaseAbsPath)
	// read database
	databaseEncBin, err := os.ReadFile(databaseAbsPath)
	if err != nil {
		common.Logger.Info("Could not read database from file.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	originalCleanupDB, err := cryptutils.XChacha20Decrypt(hPwdBytes, databaseEncBin)
	if err != nil {
		common.Logger.Info("Could not decrypt database.")
		telemetry.CaptureException(err, "MainDecryptCleanupDB")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	var cleanupDBObj = &common.CRAMCCleanupDB{}
	err = json.Unmarshal(originalCleanupDB, cleanupDBObj)
	if err != nil {
		common.Logger.Info("Could not deserialize cleanup DB.")
		common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
		os.Exit(-1)
	}
	common.Logger.Info("Successfully loaded cleanup DB.")
	common.CleanupDB = cleanupDBObj
	// dry run is always handled by callee to make sure behavior consistent.
	common.DryRunOnly = *flDryRun
	common.EnableHardening = *flEnableHardening
	// record start
	telemetry.CaptureMessage("info", "Program successfully started.")
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
			if latestV.DatabaseVersion != common.CleanupDB.Version {
				common.Logger.Log(context.TODO(), logging.LevelFatal, "Database UpdCheck: "+customerrs.ErrNotLatestVersion.Error())
				os.Exit(-1)
			}
		}
	}
	// check privilege
	isElevated, _ := fileutils.CheckProcessElevated()
	common.IsElevated = isElevated
	isNTFS, _ := fileutils.IsDriveFileSystemNTFS(*flActionPath)
	common.Logger.Info("Privilege and platform check passed.")
	// if noDiskScan set, directly go for yara scanner
	var wg = &sync.WaitGroup{}
	// searcher output channel
	var searcherOptChan = make(chan string)
	// to-process files list
	var searcherFoundList = []string{}
	var searcherFoundListRWLock = &sync.Mutex{}
	var searchConsumer = func() {
		// search result process
		for item := range searcherOptChan {
			common.Logger.Info("Found file: " + item)
			fExistsOnDisk, fSize, _ := fileutils.CheckFileOnDiskSize(item)
			if !fExistsOnDisk || fSize <= 0 {
				common.Logger.Info("File Not On Local Disk, Ignore: " + item)
				continue
			}
			searcherFoundListRWLock.Lock()
			searcherFoundList = append(searcherFoundList, item)
			searcherFoundListRWLock.Unlock()
		}
		common.Logger.Info("searchConsumer finished.")
	}
	// matched files , aio output queue
	var scanMatchedFiles = make(chan *common.YaraScanResult)
	// searcher procedure
	if !*flNoDiskScan {
		triggeredErrFallback := false
		goesForPrivileged := false
		// check if booster could be used
		if isElevated && isNTFS && !*flNoPriv {
			goesForPrivileged = true
			// prepare consumer, and check physically exists on disk
			wg.Add(1)
			go func() {
				defer wg.Done()
				searchConsumer()
			}()
			// go for parse MFT
			wg.Add(1)
			go func() {
				defer wg.Done()
				common.Logger.Info("Start MFTSearcher, BOOSTED!")
				countedFile, err := fileutils.ExtractAndParseMFTThenSearch(*flActionPath, allowedExts, searcherOptChan)
				if errors.Is(err, customerrs.ErrInvalidInput) {
					common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
					os.Exit(5)
					return
				}
				if errors.Is(err, customerrs.ErrFallbackToCompatibleSolution) || errors.Is(err, customerrs.ErrUnsupportedPlatform) {
					common.Logger.Error("Unwanted things happened using MFTSearcher, fallback.")
					triggeredErrFallback = true
					return
				}
				if err != nil {
					common.Logger.Error("Unknown error happened: " + err.Error())
					telemetry.CaptureException(err, "MFTSearcher")
					common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrUnknownInternalError.Error())
					os.Exit(2)
				}
				common.Logger.Info(fmt.Sprintf("MFTSearcher found %d applicable files.", countedFile))
				return
			}()
		} else {
			// unprivileged searcher
			if common.IsRunningOnWin {
				common.Logger.Info("Unprivileged scan, fallback.")
				triggeredErrFallback = true
			}
		}
		// must wait as you don't know when `triggeredErrFallback` will be modified
		wg.Wait()

		// unsupported platform OR MFTSearcher failed on windows
		if !common.IsRunningOnWin || triggeredErrFallback || *flNoPriv {
			// rebuild writer chan by closing and re-creating, to prevent writing on closed chan
			if !goesForPrivileged {
				close(searcherOptChan)
			}
			searcherOptChan = make(chan string)
			// had to start consumer again, because that channel got recreated
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
					telemetry.CaptureException(err, "GenrealWalkthroughSearcher")
					common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrUnknownInternalError.Error())
					os.Exit(1)
				}
				common.Logger.Info(fmt.Sprintf("Found %d File using GeneralSearcher, proceed to next step.", counted))
				return
			}()
		}
		// wait until iteration finish
		wg.Wait()
	}
	// implement sanitizer for windows only
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = sanitizer_ole.StartSanitizer()
		if errors.Is(err, customerrs.ErrUnsupportedPlatform) {
			common.Logger.Info("Due to the nature of OLE, we can only support this on Windows. Aborting for sanitization.")
		} else if err != nil {
			telemetry.CaptureException(err, "MainStartSanitizer")
			common.Logger.Error("Unknown Internal Error Happened in Sanitizer: " + err.Error())
		}
		common.Logger.Info("Sanitizer finished.")
	}()
	// start hardener server
	if *flEnableHardening && common.IsRunningOnWin {
		wg.Add(1)
		go func() {
			defer wg.Done()
			common.Logger.Debug("DEBUG: Hardener goroutine started, waiting for HardeningQueue")
			// hardener build
			for tHarden := range common.HardeningQueue {
				common.Logger.Debug("DEBUG: Hardener received request for: " + tHarden.Name)
				err := hardener.DispatchHardenAction(tHarden)
				if err != nil {
					common.Logger.Error("While hardening: " + err.Error())
				}
				common.Logger.Debug("DEBUG: Hardener completed request for: " + tHarden.Name)
			}
			common.Logger.Debug("DEBUG: Hardener goroutine exiting - HardeningQueue closed")
			common.Logger.Info("Hardening finished.")
		}()
	} else {
		common.Logger.Info("Hardening server won't start as disabled by user/running on unsupported platform.")
	}
	// retrieving scanner result async, dispatch to hardener and sanitizer queue
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(common.SanitizeQueue)
		defer close(common.HardeningQueue)
		// handle every ScanMatchedFile
		for f := range scanMatchedFiles {
			// unstable processing, if multiple detection happened on the same file
			// this will lead to undetermined processing result, possibly conflict.
			//
			// dispatch to hardener and sanitizer
			foundSolu := false
			for _, solu := range cleanupDBObj.Solutions {
				if solu.Name == f.DetectedRule {
					foundSolu = true
					tmpSanitz := &common.IPCSingleDocToBeSanitized{
						Path:          f.FilePath,
						Action:        solu.Action,
						DestModule:    solu.DestModule,
						DetectionName: f.DetectedRule,
					}
					common.SanitizeQueue <- tmpSanitz
					common.Logger.Info("Sanitizer Req Sent: " + f.FilePath + " , Detection: " + f.DetectedRule)
					if *flEnableHardening {
						// dry run handled in callee
						tmpHarden := &common.HardeningAction{
							Name:                f.DetectedRule,
							ActionLst:           solu.HardenMeasures,
							AllowRepeatedHarden: solu.AllowRepeatedHarden,
						}
						common.Logger.Debug("DEBUG: About to send to HardeningQueue for: " + f.DetectedRule)
						common.HardeningQueue <- tmpHarden //deadlock
						common.Logger.Debug("DEBUG: Successfully sent to HardeningQueue for: " + f.DetectedRule)
						common.Logger.Info("Hardener Req Sent: " + f.FilePath + " , Detection: " + f.DetectedRule)
					} else {
						common.Logger.Info("EnableHardening flag had been disabled by user.")
					}
				}
				continue
			}
			// if not match, it's abandoned, warn.
			if !foundSolu {
				common.Logger.Warn("Can't find solution for rule: " + f.DetectedRule)
			}
		}
		common.Logger.Info("Hardener&Sanitizer Request Sent finished.")
	}()
	// searcher finished, go for yara scanner
	// read yara rules and decrypt
	if !*flNoDiskScan {
		// if no diskscan, supplied output already included necessary detection information, directly go for sanitizer and hardener
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
		yrRuleBin, err := cryptutils.XChacha20Decrypt(hPwdBytes, yrRulesEncBin)
		if err != nil {
			common.Logger.Info("Could not decrypt yara compiled rules file.")
			telemetry.CaptureException(err, "MainDecryptYaraRules")
			common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
			os.Exit(-1)
		}
		// build scanner instance
		yrScanner, err := yarax_scanner.LoadRuleAndCreateYaraScanner(yrRuleBin)
		if err != nil {
			common.Logger.Info("Unable to create yara scanner with provided rule.")
			telemetry.CaptureException(err, "MainLoadRuleAndCreateYaraScanner")
			common.Logger.Log(context.TODO(), logging.LevelFatal, err.Error())
			os.Exit(-1)
		}
		common.Logger.Info("Yara scanner loaded successfully.")
		// producer set
		// go to scan against rules
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = yarax_scanner.ScanFilesWithYara(yrScanner, searcherFoundList, scanMatchedFiles)
			if err != nil {
				common.Logger.Error("Yara scanner returned err when exit: " + err.Error())
			}
			common.Logger.Info("Yara scanner finished.")
		}()
	} else {
		// iterate finished, searcher finished, now parse existing result.
		//
		// 000000b0: --0a 5669 7275 7358 3937 4d53 6c61 636b  -.VirusX97MSlack
		// 000000c0: 6572 4620 2e2f 426f 6f6b 310a ---- ----  erF ./Book1.
		// output as above: VirusX97MSlackerF ./Book1\n
		// read iptYRList
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = yarax_scanner.ParseYaraScanResultText(iptFileList, scanMatchedFiles)
			if err != nil {
				common.Logger.Error(err.Error())
				common.Logger.Log(context.TODO(), logging.LevelFatal, customerrs.ErrUnknownInternalError.Error())
				os.Exit(1)
			}
			common.Logger.Info("Yara Result Processor finished.")
		}()
	}
	// wait for all procedures
	wg.Wait()
	// wait for 5 seconds for cleanup
	runtime.GC()
	time.Sleep(5 * time.Second)
	common.Logger.Info("All done. Program exited.")
}
