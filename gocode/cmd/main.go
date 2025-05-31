//go:generate go-winres make --product-version=git-tag

package main

import (
	"cramc_go/common"
	"cramc_go/logging"
	"flag"
	"github.com/getsentry/sentry-go"
)

const (
	SentryDSN = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872"
)

var (
	flActionPath      = flag.String("actionPath", "C:\\Users", "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. By default, we use recursive search.")
	flDryRun          = flag.Bool("dryRun", false, "Scan only, take no action on files, record action to be taken in log.")
	flEnableHardening = flag.Bool("enableHardening", true, "Enables hardening measure to prevent further infection. Windows OS only.")
	flNoDiskScan      = flag.Bool("noDiskScan", false, "Do not scan files on disk, but supply file list. If platform is not Windows x86_64, yara won't work, you have to set this to true and then run Yara scanner against our rules and save output to ipt_yrscan.lst. Yara-X scanner is not supported yet.")
)

func init() {
	flag.Parse()
}

func main() {
	// init logging
	logger, logfd := logging.NewLogger()
	common.Logger = logger
	defer logfd.Sync()
	defer logfd.Close()

	// enable sentry
	err := sentry.Init(sentry.ClientOptions{
		Dsn:            SentryDSN,
		EnableTracing:  true,
		SendDefaultPII: true,
	})
	if err != nil {
		logger.Fatalf("sentry.init: %s", err)
	}
	// startup behavior

}
