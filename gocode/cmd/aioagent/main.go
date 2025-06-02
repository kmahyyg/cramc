//go:generate go-winres make --product-version=git-tag

package main

import (
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"cramc_go/platform/windoge_utils"
	"cramc_go/updchecker"
	"flag"
	"github.com/getsentry/sentry-go"
	"os"
	"sync"
)

const (
	SentryDSN     = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872"
	databasePath  = "cramc_db.bin"
	yaraRulesPath = "unified.yar.bin"
	iptFileList   = "ipt_yrscan.lst"
)

var (
	flActionPath      = flag.String("actionPath", "C:\\Users", "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. By default, we use recursive search.")
	flDryRun          = flag.Bool("dryRun", false, "Scan only, take no action on files, record action to be taken in log.")
	flEnableHardening = flag.Bool("enableHardening", true, "Enables hardening measure to prevent further infection. Windows OS only.")
	flNoDiskScan      = flag.Bool("noDiskScan", false, "Do not scan files on disk, but supply file list. If platform is not Windows x86_64, yara won't work, you have to set this to true and then run Yara scanner against our rules and save output to ipt_yrscan.lst. Yara-X scanner is not supported yet.")
	allowedExts       = []string{".xls", ".xlsx", ".xlsm", ".xlsb"}
)

func init() {
	flag.Parse()
}

func main() {
	// init logging
	logger, logfd := logging.NewLogger("cramc_go.log")
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
	// read and decrypt file

	// dry run is always handled by callee to make sure behaviour consistent.
	common.DryRunOnly = *flDryRun
	common.EnableHardening = *flEnableHardening
	// kill office process on windows
	_, _ = windoge_utils.KillAllOfficeProcesses()
	// update checker
	latestV, err := updchecker.CheckUpdateFromInternet()
	if err != nil {
		logger.Errorln("Update Checker Error: ", err.Error())
	} else {
		if latestV.ProgramRevision != common.ProgramRev {
			logger.Fatalln(customerrs.ErrNotLatestVersion)
		}
	}
	// check privilege
	isElevated, _ := fileutils.CheckProcessElevated()
	isNTFS, _ := fileutils.IsDriveFileSystemNTFS(*flActionPath)
	// if noDiskScan set, directly go for yara scanner
	var wg = &sync.WaitGroup{}
	if !*flNoDiskScan {
		if isElevated && isNTFS {
			// go for parse MFT
		} else {
			// fallback to normal searcher
		}
	} else {
		// 000000b0: 6e0a 5669 7275 7358 3937 4d53 6c61 636b  n.VirusX97MSlack
		// 000000c0: 6572 4620 2e2f 426f 6f6b 310a            erF ./Book1.
		// output as above: VirusX97MSlackerF ./Book1\n
	}
}
