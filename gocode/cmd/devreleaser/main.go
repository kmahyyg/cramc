package main

import (
	"cramc_go/common"
	"cramc_go/logging"
	"flag"
)

var (
	fEnc  = flag.Bool("enc", false, "encrypt")
	fComp = flag.Bool("compile", false, "compile yara rules")
	fDec  = flag.Bool("dec", false, "decrypt")
)

const (
	databaseName   = "cramc_db.json"
	mergedYaraName = "merged.yar"
)

func init() {
	flag.Parse()
}

func main() {
	logger, logFd := logging.NewLogger()
	defer logFd.Sync()
	defer logFd.Close()
	common.Logger = logger
	logger.Infoln("DevReleaser for CRAMC, Don't Forget to Bump Database/UpdateChecker Version!")
	logger.Infoln("Current Version: ", common.VersionStr)
	logger.Infoln("Please put this binary with the same folder of yrules/ and cramc_db.json before continue.")
	if *fComp {

	}
	if *fDec {

	}
	if *fEnc {

	}
}
