//go:generate go-winres make --product-version=git-tag

package main

import (
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
	flNoPriv          = flag.Bool("nopriv", false, "Do not run as privileged user, even you are privileged.")
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
	logger.Infoln("Welcome to CRAMC!")
	logger.Infoln("Current Version: ", common.VersionStr)
	if finfo, err := os.Stat(*flActionPath); err != nil || !finfo.IsDir() {
		logger.Fatalln(customerrs.ErrActionPathMustBeDir)
	}
	if *flNoDiskScan {
		iptFinfo, err := os.Stat(iptFileList)
		if err != nil || iptFinfo.IsDir() {
			logger.Fatalln(customerrs.ErrNoScanSetButNoListProvided)
		}
	}
	common.Logger.Infoln("Initial args-check passed.")
	// read and decrypt config file
	hPwdBytes, err := hex.DecodeString(common.HexEncryptionPassword)
	if err != nil {
		logger.Infoln("Cannot prepare password.")
		logger.Fatalln(err)
	}
	// fix #9
	execPath, err := os.Executable()
	if err != nil {
		logger.Infoln("Cannot get executable path.")
		logger.Fatalln(err)
	}
	execDir := filepath.Dir(execPath)
	databaseAbsPath := filepath.Join(execDir, databasePath)
	common.Logger.Debugln("DEBUG: Database path: ", databaseAbsPath)
	// read database
	databaseEncBin, err := os.ReadFile(databaseAbsPath)
	if err != nil {
		logger.Infoln("Could not read database from file.")
		logger.Fatalln(err)
	}
	originalCleanupDB, err := cryptutils.XChacha20Decrypt(hPwdBytes, databaseEncBin)
	if err != nil {
		logger.Infoln("Could not decrypt database.")
		telemetry.CaptureException(err, "MainDecryptCleanupDB")
		logger.Fatalln(err)
	}
	var cleanupDBObj = &common.CRAMCCleanupDB{}
	err = json.Unmarshal(originalCleanupDB, cleanupDBObj)
	if err != nil {
		logger.Infoln("Could not deserialize cleanup DB.")
		logger.Fatalln(err)
	}
	common.Logger.Infoln("Successfully loaded cleanup DB.")
	common.CleanupDB = cleanupDBObj
	// dry run is always handled by callee to make sure behavior consistent.
	common.DryRunOnly = *flDryRun
	common.EnableHardening = *flEnableHardening
	// record start
	telemetry.CaptureMessage("info", "Program successfully started.")
	// kill M365 office processes on windows
	_, _ = windoge_utils.KillAllOfficeProcesses()
	common.Logger.Infoln("Triggered M365 Office processes killer.")
	// update checker
	latestV, err := updchecker.CheckUpdateFromInternet()
	if err != nil {
		logger.Errorln("Update Checker Error: ", err.Error())
	} else {
		if latestV.ProgramRevision != common.ProgramRev {
			logger.Fatalln("Program UpdCheck: ", customerrs.ErrNotLatestVersion)
		}
		if latestV.DatabaseVersion != common.CleanupDB.Version {
			logger.Fatalln("Database UpdCheck: ", customerrs.ErrNotLatestVersion)
		}
	}
	common.Logger.Infoln("Called update-checker.")
	// check privilege
	isElevated, _ := fileutils.CheckProcessElevated()
	common.IsElevated = isElevated
	isNTFS, _ := fileutils.IsDriveFileSystemNTFS(*flActionPath)
	common.Logger.Infoln("Privilege and platform check passed.")
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
			common.Logger.Infoln("Found file: ", item)
			fExistsOnDisk, fSize, _ := fileutils.CheckFileOnDiskSize(item)
			if !fExistsOnDisk || fSize <= 0 {
				common.Logger.Infoln("File Not On Local Disk, Ignore: ", item)
				continue
			}
			searcherFoundListRWLock.Lock()
			searcherFoundList = append(searcherFoundList, item)
			searcherFoundListRWLock.Unlock()
		}
		common.Logger.Infoln("searchConsumer finished.")
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
				common.Logger.Infoln("Start MFTSearcher, BOOSTED!")
				countedFile, err := fileutils.ExtractAndParseMFTThenSearch(*flActionPath, allowedExts, searcherOptChan)
				if errors.Is(err, customerrs.ErrInvalidInput) {
					common.Logger.Fatalln(err)
					return
				}
				if errors.Is(err, customerrs.ErrFallbackToCompatibleSolution) || errors.Is(err, customerrs.ErrUnsupportedPlatform) {
					common.Logger.Errorln("Unwanted things happened using MFTSearcher, fallback.")
					triggeredErrFallback = true
					return
				}
				if err != nil {
					common.Logger.Errorln("Unknown error happened: ", err)
					telemetry.CaptureException(err, "MFTSearcher")
					common.Logger.Fatalln(customerrs.ErrUnknownInternalError)
				}
				common.Logger.Infof("MFTSearcher found %d applicable files.", countedFile)
				return
			}()
		} else {
			// unprivileged searcher
			if common.IsRunningOnWin {
				common.Logger.Infoln("Unprivileged scan, fallback.")
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
				common.Logger.Infoln("GenrealWalkthroughSearcher started.")
				counted, err := fileutils.GeneralWalkthroughSearch(*flActionPath, allowedExts, searcherOptChan)
				// should not encounter some unexpected error
				if err != nil {
					common.Logger.Errorln("Unwanted error in GeneralSearcher: ", err)
					telemetry.CaptureException(err, "GenrealWalkthroughSearcher")
					common.Logger.Fatalln(customerrs.ErrUnknownInternalError)
				}
				common.Logger.Infof("Found %d File using GeneralSearcher, proceed to next step.", counted)
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
			common.Logger.Infoln("Due to the nature of OLE, we can only support this on Windows. Aborting for sanitization.")
		} else if err != nil {
			telemetry.CaptureException(err, "MainStartSanitizer")
			common.Logger.Errorln("Unknown Internal Error Happened in Sanitizer: ", err.Error())
		}
		common.Logger.Infoln("Sanitizer finished.")
	}()
	// start hardener server
	if *flEnableHardening && common.IsRunningOnWin {
		wg.Add(1)
		go func() {
			defer wg.Done()
			common.Logger.Debugln("DEBUG: Hardener goroutine started, waiting for HardeningQueue")
			// hardener build
			for tHarden := range common.HardeningQueue {
				common.Logger.Debugln("DEBUG: Hardener received request for: ", tHarden.Name)
				err := hardener.DispatchHardenAction(tHarden)
				if err != nil {
					common.Logger.Errorln("While hardening: ", err)
				}
				common.Logger.Debugln("DEBUG: Hardener completed request for: ", tHarden.Name)
			}
			common.Logger.Debugln("DEBUG: Hardener goroutine exiting - HardeningQueue closed")
			common.Logger.Infoln("Hardening finished.")
		}()
	} else {
		common.Logger.Infoln("Hardening server won't start as disabled by user/running on unsupported platform.")
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
					tmpSanitz := &common.IpcSingleDocToBeSanitized{
						Path:          f.FilePath,
						Action:        solu.Action,
						DestModule:    solu.DestModule,
						DetectionName: f.DetectedRule,
					}
					common.SanitizeQueue <- tmpSanitz
					common.Logger.Infoln("Sanitizer Req Sent: ", f.FilePath, " ,Detection: ", f.DetectedRule)
					if *flEnableHardening {
						// dry run handled in callee
						tmpHarden := &common.HardeningAction{
							Name:                f.DetectedRule,
							ActionLst:           solu.HardenMeasures,
							AllowRepeatedHarden: solu.AllowRepeatedHarden,
						}
						common.Logger.Debugln("DEBUG: About to send to HardeningQueue for: ", f.DetectedRule)
						common.HardeningQueue <- tmpHarden //deadlock
						common.Logger.Debugln("DEBUG: Successfully sent to HardeningQueue for: ", f.DetectedRule)
						common.Logger.Infoln("Hardener Req Sent: ", f.FilePath, " ,Detection: ", f.DetectedRule)
					} else {
						common.Logger.Infoln("EnableHardening flag had been disabled by user.")
					}
				}
				continue
			}
			// if not match, it's abandoned, warn.
			if !foundSolu {
				common.Logger.Warnln("Can't find solution for rule: ", f.DetectedRule)
			}
		}
		common.Logger.Infoln("Hardener&Sanitizer Request Sent finished.")
	}()
	// searcher finished, go for yara scanner
	// read yara rules and decrypt
	if !*flNoDiskScan {
		// if no diskscan, supplied output already included necessary detection information, directly go for sanitizer and hardener
		// fix #9
		yaraRulesAbsPath := filepath.Join(execDir, yaraRulesPath)
		common.Logger.Debugln("DEBUG: Yara Rules path: ", yaraRulesAbsPath)
		yrRulesEncBin, err := os.ReadFile(yaraRulesAbsPath)
		if err != nil {
			logger.Infoln("Could not read yara compiled rules file.")
			logger.Fatalln(err)
		}
		common.Logger.Infoln("Compiled yara rules read.")
		yrRuleBin, err := cryptutils.XChacha20Decrypt(hPwdBytes, yrRulesEncBin)
		if err != nil {
			logger.Infoln("Could not decrypt yara compiled rules file.")
			telemetry.CaptureException(err, "MainDecryptYaraRules")
			logger.Fatalln(err)
		}
		// build scanner instance
		yrScanner, err := yarax_scanner.LoadRuleAndCreateYaraScanner(yrRuleBin)
		if err != nil {
			logger.Infoln("Unable to create yara scanner with provided rule.")
			telemetry.CaptureException(err, "MainLoadRuleAndCreateYaraScanner")
			logger.Fatalln(err)
		}
		common.Logger.Infoln("Yara scanner loaded successfully.")
		// producer set
		// go to scan against rules
		wg.Add(1)
		go func() {
			defer wg.Done()
			err = yarax_scanner.ScanFilesWithYara(yrScanner, searcherFoundList, scanMatchedFiles)
			if err != nil {
				common.Logger.Errorln("Yara scanner returned err when exit: ", err)
			}
			common.Logger.Infoln("Yara scanner finished.")
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
				common.Logger.Errorln(err)
				common.Logger.Fatalln(customerrs.ErrUnknownInternalError)
			}
			common.Logger.Infoln("Yara Result Processor finished.")
		}()
	}
	// wait for all procedures
	wg.Wait()
	// wait for 5 seconds for cleanup
	runtime.GC()
	time.Sleep(5 * time.Second)
	common.Logger.Infoln("All done. Program exited.")
}
