package main

import (
	"context"
	"cramc_go/common"
	"cramc_go/cryptutils"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"cramc_go/yarax_scanner"
	"encoding/hex"
	"flag"
	"os"
)

var (
	fEnc     = flag.Bool("enc", false, "encrypt")
	fComp    = flag.Bool("compile", false, "compile yara rules")
	fDec     = flag.Bool("dec", false, "decrypt")
	fInFile  = flag.String("in", "unset-placeholder", "input file")
	fOutFile = flag.String("out", "unset-placeholder", "output file")
)

const (
	databaseName = "cramc_db.json"
)

func init() {
	flag.Parse()
}

func main() {
	logger, logFd := logging.NewLogger("cramc_go_devrel.log")
	defer logFd.Close()
	defer logFd.Sync()
	common.Logger = logger
	common.Logger.Info("DevReleaser for CRAMC, Don't Forget to Bump Database/UpdateChecker Version!")
	common.Logger.Info("Current Version: " + common.VersionStr)
	common.Logger.Info("Please put this binary with the same folder of yrules/ and cramc_db.json before continue.")
	if *fComp {
		err := os.MkdirAll("yrules/bin/", 0755)
		if err != nil {
			panic(err)
		}
		common.Logger.Info("Binary rules folder created!")
		_ = os.Remove("yrules/bin/unified.yar")
		common.Logger.Info("Tried to remove previously compiled rules!")
		yarax_scanner.MergeAndCompile2UnifiedRules("yrules/", "yrules/bin/unified.yar")
		common.Logger.Info("Operation finished.")
		return
	}
	if *fDec && *fEnc {
		common.Logger.Info("Conflicted flag supplied. Refuse to continue.")
		panic(customerrs.ErrInvalidInput)
	}
	if *fDec || *fEnc {
		if !fileutils.CheckFileLogicalExists(*fInFile) {
			common.Logger.Log(context.TODO(), logging.LevelFatal, "Input file not found"+*fInFile)
			panic(customerrs.ErrInvalidInput)
		}
		inData, err := os.ReadFile(*fInFile)
		if err != nil {
			panic(err)
		}
		common.Logger.Info("Input file Read Into Memory: " + *fInFile)
		outFd, err := os.Create(*fOutFile)
		if err != nil {
			panic(err)
		}
		defer outFd.Close()
		common.Logger.Info("Output file Created: " + *fOutFile)
		passwd, err := hex.DecodeString(common.HexEncryptionPassword)
		if err != nil {
			panic(err)
		}
		common.Logger.Info("Password Prepared.")
		var outD []byte
		if *fEnc {
			outD, err = cryptutils.XChacha20Encrypt(passwd, inData)
			if err != nil {
				panic(err)
			}
			common.Logger.Info("Input file Encrypted.")
		} else if *fDec {
			outD, err = cryptutils.XChacha20Decrypt(passwd, inData)
			if err != nil {
				panic(err)
			}
			common.Logger.Info("Input file Decrypted.")
		}
		_, err = outFd.Write(outD)
		if err != nil {
			panic(err)
		}
		outFd.Sync()
		common.Logger.Info("Successfully wrote output binary!")
		return
	}
}
