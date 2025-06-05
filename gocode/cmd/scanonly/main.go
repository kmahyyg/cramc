package main

import (
	"cramc_go/common"
	"cramc_go/cryptutils"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"cramc_go/platform/windoge_utils"
	"cramc_go/yara_scanner"
	"encoding/hex"
	"flag"
	"github.com/getsentry/sentry-go"
	"os"
	"sync"
)

var (
	flActionPath = flag.String("actionPath", "C:\\Users", "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. By default, we use recursive search.")
	allowedExts  = []string{".xls", ".xlsx", ".xlsm", ".xlsb"}
	flDryRun     = flag.Bool("dryRun", false, "Scan only, take no action on files, record action to be taken in log.")
)

const (
	yaraRulesPath = "unified.yar.bin"
	SentryDSN     = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872"
)

func main() {
	logger, logfd := logging.NewLogger("cramc_go_scanonly.log")
	common.Logger = logger
	defer logfd.Close()
	defer logfd.Sync()

	// enable sentry
	err := sentry.Init(sentry.ClientOptions{
		Dsn:            SentryDSN,
		EnableTracing:  true,
		SendDefaultPII: true,
	})
	if err != nil {
		logger.Fatalf("sentry.init failed: %s", err)
	}

	// dry run is always handled by callee to make sure behavior consistent.
	common.DryRunOnly = *flDryRun
	// startup behavior
	logger.Infoln("Welcome to CRAMC Scanner!")
	logger.Infoln("Current Version: ", common.VersionStr)
	if finfo, err := os.Stat(*flActionPath); err != nil || !finfo.IsDir() {
		logger.Fatalln(customerrs.ErrActionPathMustBeDir)
	}
	logger.Infoln("Initial args-check passed.")
	// read and decrypt yara rule file
	hPwdBytes, err := hex.DecodeString(common.HexEncryptionPassword)
	if err != nil {
		logger.Infoln("Cannot prepare password.")
		logger.Fatalln(err)
	}
	yrRulesEncBin, err := os.ReadFile(yaraRulesPath)
	if err != nil {
		logger.Infoln("Could not read yara compiled rules file.")
		logger.Fatalln(err)
	}
	common.Logger.Infoln("Compiled yara rules read.")
	yrRuleBin, err := cryptutils.XChacha20Decrypt(hPwdBytes, yrRulesEncBin)
	if err != nil {
		logger.Infoln("Could not decrypt yara compiled rules file.")
		logger.Fatalln(err)
	}
	// build scanner instance
	yrScanner, err := yara_scanner.LoadRuleAndCreateYaraScanner(yrRuleBin)
	if err != nil {
		logger.Infoln("Unable to create yara scanner with provided rule.")
		logger.Fatalln(err)
	}
	// make sure memory won't leak
	defer yara_scanner.RecycleYaraResources()
	common.Logger.Infoln("Yara scanner loaded successfully.")
	// kill M365 office processes on windows
	_, _ = windoge_utils.KillAllOfficeProcesses()
	// matched files , aio output queue
	var scanMatchedFiles = make(chan *common.YaraScanResult)
	// searcher procedure
	var searcherOptChan = make(chan string)
	// async concurrent control
	var wg = &sync.WaitGroup{}
	// prepare for iterate
	var searcherFoundList = []string{}
	var searcherFoundListRWLock = &sync.Mutex{}
	var searchConsumer = func() {
		for item := range searcherOptChan {
			fExistsOnDisk, fSize, _ := fileutils.CheckFileOnDiskSize(item)
			if !fExistsOnDisk || fSize <= 0 {
				common.Logger.Infoln("File Not On Local Disk, Ignore: ", item)
				continue
			}
			searcherFoundListRWLock.Lock()
			searcherFoundList = append(searcherFoundList, item)
			searcherFoundListRWLock.Unlock()
		}
	}
	// prepare for scan
	virusLogger, virusResfd := logging.NewLogger("cramc_go_scanresult.log")
	defer virusResfd.Close()
	defer virusResfd.Sync()
	var yaraResultConsumer = func() {
		for item := range scanMatchedFiles {
			virusLogger.Infof("Matched rule %s with file: %s ", item.DetectedRule, item.FilePath)
		}
	}
	// start consumers
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
			common.Logger.Fatalln(customerrs.ErrUnknownInternalError)
		}
		common.Logger.Infof("Found %d File using GeneralSearcher, proceed to next step.\n", counted)
		return
	}()
	wg.Wait()
	// init yara scanner
	// producer set
	// go to scan against rules
	wg.Add(1)
	go yaraResultConsumer()
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = yara_scanner.ScanFilesWithYara(yrScanner, searcherFoundList, scanMatchedFiles)
		if err != nil {
			common.Logger.Fatalln(err)
		}
	}()
	// wait all
	wg.Wait()
}
