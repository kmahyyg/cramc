package fileutils

import (
	"archive/zip"
	"cramc_go/common"
	"io"
	"os"
)

func DecompressMacroBin(fPath string) ([]byte, error) {
	zRd, err := zip.OpenReader(fPath)
	if err != nil {
		return nil, err
	}
	defer zRd.Close()
	vbaProjFile, err := zRd.Open("xl/vbaProject.bin")
	if err != nil {
		common.Logger.Info("Unable to find vbaProject.bin, ignore.")
		return nil, err
	}
	fBytes, err := io.ReadAll(vbaProjFile)
	return fBytes, err
}

func ReplaceXLVBAProjectBin(fPath string, modifiedBytes []byte) error {
	newfPath := fPath + ".tmp"

	zrrd, err := zip.OpenReader(fPath)
	if err != nil {
		return err
	}
	defer zrrd.Close()

	zwf, err := os.Create(newfPath)
	if err != nil {
		return err
	}
	defer zwf.Close()
	zwr := zip.NewWriter(zwf)
	if err != nil {
		return err
	}
	defer zwr.Close()
	defer zwr.Flush()

	for _, zipEntry := range zrrd.File {
		if zipEntry.Name == "xl/vbaProject.bin" {
			continue
		}
		zitemRdr, err := zipEntry.OpenRaw()
		if err != nil {
			return err
		}
		zitemHdr := zipEntry.FileHeader
		targetItem, err := zwr.CreateRaw(&zitemHdr)
		if err != nil {
			return err
		}
		_, err = io.Copy(targetItem, zitemRdr)
		if err != nil {
			return err
		}
	}
	common.Logger.Info("Unmodified zip file entries copied from old file successfully.")

	vbaWr, err := zwr.Create("xl/vbaProject.bin")
	if err != nil {
		return err
	}
	_, err = vbaWr.Write(modifiedBytes)
	if err != nil {
		return err
	}
	common.Logger.Info("Modified vbaProject.bin wrote to zip container successfully.")
	return nil
}
