using System;
using System.Collections.Generic;
using System.Threading.Channels;
using CRAMC.Common;
using System.IO;
using Serilog;
using RawCopy;
using MFT;
using MFT.Other;
using MFT.Attributes;
using System.Linq;
using System.Threading.Tasks;

namespace CRAMC.FileUtils;

public class NTFSMFTSearch : IFileSearcher {
    public long FindMatchedFilesUnderPath(string actionPath, string[] allowedExts, Channel<string> matchedFileOutputChan) {
        // read and parse MFT from disk
        if (RuntimeOpts.NoPrivilegedActions || !RuntimeOpts.IsWindows) {
            Log.Error("Failed platform check in CreateMFTMemoryMap.");
            throw new Exception("No Privileged actions set.");
        }
        string diskDrivePath = actionPath.Substring(0, 3).ToUpperInvariant();
        var drvInfo = DriveInfo.GetDrives();
        var ntfsConfirmed = false;
        foreach (DriveInfo drive in drvInfo) {
            if (drive.Name.ToUpperInvariant() == diskDrivePath && drive.DriveFormat == "NTFS") {
                Log.Information("Found NTFS drive match the disk drive letter.");
                ntfsConfirmed = true;
                break;
            }
        }

        if (ntfsConfirmed) {
            var fNameLst = new List<string> { string.Concat(diskDrivePath, "$MFT") };
            var rawFiles = Helper.GetRawFiles(fNameLst);
            
            // When ntfsConfirmed == true, there should only match one file
            if (rawFiles.Count != 1) {
                Log.Error($"Expected exactly one MFT file, but found {rawFiles.Count} files.");
                throw new Exception($"Expected exactly one MFT file, but found {rawFiles.Count} files.");
            }
            var r = rawFiles[0];
            var fileStream = r.FileStream;
            long fileSize = fileStream.Length;
            
            // MFT file may be larger than 2GB, in my own disk, it's around 2.3GB
            Log.Debug($"MFT file size: {fileSize} bytes ({fileSize / (1024.0 * 1024.0):F2} MB)");
            
            // Parse MFT file and check, if file on disk matched any of the allowedExts,
            // concat disk drive to generate file full path and write to matchedFileOutputChan
            try {
                Log.Information("Starting MFT parsing...");
                var mft = new Mft(fileStream, false);
                long matchedFileCount = 0;
                
                // Convert allowed extensions to lowercase for case-insensitive comparison
                var allowedExtsLower = allowedExts.Select(ext => ext.ToLowerInvariant()).ToHashSet();
                
                foreach (var fileRecord in mft.FileRecords.Values) {
                    // Skip deleted files and directories
                    if (fileRecord.IsDeleted() || fileRecord.IsDirectory()) {
                        continue;
                    }
                    
                    // Get the file name from the first filename attribute
                    var fileName = fileRecord.GetFileNameAttributeFromFileRecord().Name;
                    if (string.IsNullOrEmpty(fileName)) {
                        continue;
                    }
                    
                    // Check if file extension matches allowed extensions
                    var fileExtension = Path.GetExtension(fileName).ToLowerInvariant();
                    if (allowedExtsLower.Contains(fileExtension)) {
                        // Get the full path by combining the parent directory path and filename
                        var parentPath = mft.GetFullParentPath(fileRecord.EntryNumber.ToString());
                        var fullPath = Path.Combine(parentPath, fileName);
                        
                        if (!string.IsNullOrEmpty(fullPath)) {
                            // Ensure the path starts with the correct drive letter
                            var completePath = Path.Combine(diskDrivePath.TrimEnd('\\'), fullPath.TrimStart('\\'));
                            
                            // Write to output channel
                            if (!matchedFileOutputChan.Writer.TryWrite(completePath)) {
                                Log.Warning($"Failed to write file path to output channel: {completePath}");
                            } else {
                                matchedFileCount++;
                                Log.Debug($"Found matching file: {completePath}");
                            }
                        }
                    }
                }
                
                Log.Information($"MFT parsing completed. Found {matchedFileCount} matching files.");
                return matchedFileCount;
            }
            catch (Exception ex) {
                Log.Error(ex, "Error occurred while parsing MFT file");
                return -1;
            }
            finally {
                // Dispose the raw file stream
                fileStream?.Dispose();
            }
        }
        Log.Error("This drive is not formatted as NTFS, unable to proceed with further actions.");
        throw new Exception("This drive is not formatted as NTFS, unable to proceed with further actions.");
    }
}