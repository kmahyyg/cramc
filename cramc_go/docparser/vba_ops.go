package docparser

import (
	"bytes"
	"cramc_go/common"
	"cramc_go/customerrs"
	"cramc_go/fileutils"
	"fmt"
	"io"
	"os"
	cfbv3 "vbaModifier/lib/cfb/v3"
	"vbaModifier/lib/vba"
)

const REPLACED_VBA_SRC_CODE = "' Sanitized by CRAMC v0.6.0\r\n"

func ExtractVBACode(vbaProjectBin []byte, isLegacyFormat bool) (results common.ExtractedVBAModules, err error) {
	// read vbaProject.bin
	rdr, err := vba.OpenVBAProjectBytes(vbaProjectBin, isLegacyFormat)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	common.Logger.Info("Successfully opened vbaProject.")
	// parse dir stream
	projectStreams, err := rdr.ParseAll()
	if err != nil {
		return nil, err
	}
	common.Logger.Info("Successfully parsed vbaProject.")
	// get module offest and names
	results = make(common.ExtractedVBAModules, len(projectStreams.DirStream.Modules))
	for i, m := range projectStreams.DirStream.Modules {
		// for each available module
		results[i] = &common.ExtractedVBAModule{
			ModuleName: m.Name,
			TextOffset: m.TextOffset,
		}
		common.Logger.Info("Successfully identified module: " + m.Name)
		// find raw module data bytes
		rawModule, err := rdr.GetModuleContent(m.Name)
		if err != nil {
			return nil, err
		}
		common.Logger.Info("Successfully copied raw content for module: " + m.Name)
		// find vba source code location and decompress for raw data
		rawSRC, err := vba.Decompress(rawModule[int(m.TextOffset):])
		if err != nil {
			return nil, err
		}
		common.Logger.Info("Successfully extracted raw source code of vba module: " + m.Name)
		results[i].SourceCode = rawSRC
	}
	common.Logger.Info(fmt.Sprintf("Successfully extracted %d modules from vbaProject.", len(results)))
	return
}

func ReplaceMaliciousCode(originalFilePath string, modulesLst []string, isLegacyFormat bool) error {
	// Open File
	var vbaProjectBin []byte
	var err error
	// If not Legacy, extract xl/vbaProject.bin
	if !isLegacyFormat {
		vbaProjectBin, err = fileutils.DecompressMacroBin(originalFilePath)
		if err != nil {
			return err
		}
	} else {
		vbaProjectBin, err = os.ReadFile(originalFilePath)
		if err != nil {
			return err
		}
	}
	common.Logger.Info("vbaProject.bin extracted from original file.")
	// Parse DIR module
	fileRdr, err := cfbv3.OpenBytes(vbaProjectBin)
	if err != nil {
		return err
	}
	defer fileRdr.Close()
	fileRootStor, err := fileRdr.OpenRootStorage()
	if err != nil {
		return err
	}
	common.Logger.Info("Root storage opened successfully.")
	vbaStorV3, err := vba.OpenVBAStorage(fileRootStor, isLegacyFormat)
	if err != nil {
		return err
	}
	common.Logger.Info("VBA storage opened successfully.")
	rawDirStream, err := vbaStorV3.OpenStream("dir")
	if err != nil {
		return err
	}
	defer rawDirStream.Close()
	var patchedDirStreamBuf = &bytes.Buffer{}
	_, err = io.Copy(patchedDirStreamBuf, rawDirStream)
	if err != nil {
		return err
	}
	var rawModOffset []vba.ModuleTextOffsetInfo
	for i, mName := range modulesLst {
		newDirStreamBytes, newModOffset, err := vba.PatchModuleTextOffsetInDirStream(patchedDirStreamBuf.Bytes(), mName, 0)
		if i == 0 {
			rawModOffset = newModOffset
		}
		if err != nil {
			return err
		}
		patchedDirStreamBuf.Reset()
		patchedDirStreamBuf.Write(newDirStreamBytes)
	}
	common.Logger.Info(fmt.Sprintf("Patched module text offsets for %d modules, dir stream parsed succeeded.", len(modulesLst)))
	// remove __SRP_* streams
	removed := removeSRPStreams(vbaStorV3)
	common.Logger.Info(fmt.Sprintf("Removed %d __SRP_* streams.", removed))
	// remove PerformanceCache from _VBA_PROJECT
	if vbaStorV3.StreamExists("_VBA_PROJECT") {
		stripped := vba.StripVBAProjectPerformanceCache()
		err = vbaStorV3.ReplaceStream("_VBA_PROJECT", stripped)
		if err != nil {
			return err
		}
	}
	common.Logger.Info("PerformanceCache removed from _VBA_PROJECT.")
	// replace content to target
	for _, mName2 := range modulesLst {
		err = func() error {
			streamR, err := vbaStorV3.OpenStream(mName2)
			if err != nil {
				return err
			}
			defer streamR.Close()
			rawStreamBytes, err := io.ReadAll(streamR)
			if err != nil {
				return err
			}
			targetOffset := findTargetModule(rawModOffset, mName2)
			if targetOffset != nil {
				extractRawSrc, err := vba.Decompress(rawStreamBytes[int(targetOffset.TextOffset):])
				if err != nil {
					return err
				}
				srcAttr, _ := splitVBASource(extractRawSrc)
				newModuleRaw := buildModuleSource(srcAttr, REPLACED_VBA_SRC_CODE)
				newModuleCompressed, err := vba.Compress(newModuleRaw)
				if err != nil {
					return err
				}
				if len(newModuleCompressed) > len(rawStreamBytes) {
					return customerrs.ErrReplaceCodeOverflow
				}
				err = vbaStorV3.ReplaceStream(mName2, newModuleCompressed)
				if err != nil {
					return err
				}
				common.Logger.Info(fmt.Sprintf("Replaced module %s with new code.", mName2))
			} else {
				return vba.ErrDecompressionFailed
			}
			return nil
		}()
		if err != nil {
			common.Logger.Error("During replacement process, encountered error: " + err.Error())
		}
	}
	// replace modified dir
	err = vbaStorV3.ReplaceStream("dir", patchedDirStreamBuf.Bytes())
	if err != nil {
		return err
	}
	common.Logger.Info("Modified dir stream replaced successfully.")
	// write out to buf
	finalBuf := &bytes.Buffer{}
	defer finalBuf.Reset()
	err = writeModifiedCFBBytes(fileRdr, finalBuf)
	if err != nil {
		return err
	}
	// if legacy, replace whole file
	if isLegacyFormat {
		newFd, err := os.Create(originalFilePath + ".tmp")
		if err != nil {
			return err
		}
		defer newFd.Close()
		defer newFd.Sync()
		_, err = finalBuf.WriteTo(newFd)
		if err != nil {
			return err
		}
		common.Logger.Info("Replaced file wrote to destination temporary location successfully.")
	} else {
		// if modern, replace xl/vbaProject.bin entry in zip
		err = fileutils.ReplaceXLVBAProjectBin(originalFilePath, finalBuf.Bytes())
		if err != nil {
			return err
		}
		common.Logger.Info("Modified zip container wrote to destination temporary location successfully.")
	}
	return nil
}
