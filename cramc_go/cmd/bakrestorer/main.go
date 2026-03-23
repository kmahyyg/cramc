package main

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/cryptutils"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"cramc_go/logging"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/klauspost/compress/zstd"
	"io"
	"os"
)

var (
	fInFile       = flag.String("f", "", "input file")
	fShowInfoOnly = flag.Bool("s", false, "show info-only")
	fOutFile      = flag.String("o", "placeholder", "output file")
)

func init() {
	flag.Parse()
}

func main() {
	logger, logFd := logging.NewLogger("cramc_go_bakrestorer.log")
	defer logFd.Close()
	defer logFd.Sync()
	common.Logger = logger
	common.Logger.Info("CRAMC BakRestorer: ZSTD+XCHACHA20-Poly1305 Decrypter&Decompressor")
	common.Logger.Info("Current Version: " + common.VersionStr)
	common.Logger.Info(fmt.Sprintf("Bundled Schema Version: %d, Program Rev: %d", cryptutils.AddiMsgVersionInAssoData, common.ProgramRev))
	// parse associated data
	if !fileutils.CheckFileLogicalExists(*fInFile) {
		panic(customerrs.ErrInvalidInput)
	}
	if (*fOutFile) != "placeholder" && fileutils.CheckFileLogicalExists(*fOutFile) {
		panic(customerrs.ErrOutputAlreadyExists)
	}
	// read encrypted file
	oriFdData, err := os.ReadFile(*fInFile)
	if err != nil {
		panic(err)
	}
	common.Logger.Info("Read Input File Finished.")
	// show info
	if *fShowInfoOnly {
		common.Logger.Info("show-info-only set, will only display information in associated data and exit.")
		common.Logger.Warn("please note: this functionality won't verify integrity of file. stay cautious.")
		fAssoD, msgV, kCRC, amadL, amad, err2 := cryptutils.GetAssoData(oriFdData)
		if err2 != nil {
			panic(err2)
		}
		common.Logger.Info("assoData found.")
		_ = cryptutils.InterpreteAssoData(fAssoD, msgV, kCRC, amadL, amad)
		common.Logger.Info("interpreting finished, now exit.")
		return
	}
	// decrypt first
	pwdBytes, err := hex.DecodeString(common.HexEncryptionPassword)
	if err != nil {
		panic(err)
	}
	common.Logger.Info("passphrase decoded.")
	addiMsg, pt, err := cryptutils.XChacha20Decrypt(pwdBytes, oriFdData)
	if err != nil {
		panic(err)
	}
	common.Logger.Debug("addiMsg: " + string(addiMsg))
	common.Logger.Info("decryption finished.")
	// check if path in addiMsg exists
	oriDstFileExistFlag := fileutils.CheckFileLogicalExists(string(addiMsg))
	common.Logger.Debug("oriDstFileExistFlag: ", oriDstFileExistFlag)
	// decompress next
	zstdDec, err := zstd.NewReader(bytes.NewReader(pt))
	if err != nil {
		common.Logger.Error("unable to open reader of zstd with password: " + err.Error())
		panic(err)
	}
	defer zstdDec.Close()
	var curOutFPath string
	if (*fOutFile) != "placeholder" {
		common.Logger.Warn("fOutFile is set, try to create file and write to destination instead of using original path.")
		// fOutFile flag priority > original path
		curOutFPath = *fOutFile
	} else {
		if oriDstFileExistFlag {
			panic(customerrs.ErrOutputAlreadyExists)
		}
		// create file at original path as-is
		curOutFPath = string(addiMsg)
		common.Logger.Warn("use original path for exported file from associatedData as-is.")
	}
	dstF, err2 := os.Create(curOutFPath)
	if err2 != nil {
		panic(err2)
	}
	common.Logger.Info("destination output file created.")
	defer dstF.Close()
	defer dstF.Sync()
	_, err = io.Copy(dstF, zstdDec)
	if err != nil {
		panic(err)
	}
	common.Logger.Info("decompression finished.")
	return
}
