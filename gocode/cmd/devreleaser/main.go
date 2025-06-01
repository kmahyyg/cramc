package main

import (
	"cramc_go/common"
	"cramc_go/logging"
	"flag"
)

var (
	fEnc    = flag.Bool("enc", false, "encrypt")
	fTarget = flag.String("target", "", "target")
	fDec    = flag.Bool("dec", false, "decrypt")
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
	logger.Infoln("Please put this binary with the same folder of yrules/ and cramc_db.json before continue.")
	if len(*fTarget) == 0 {
		panic("target is required")
	}
	if *fDec {

	}
	if *fEnc {

	}
}
