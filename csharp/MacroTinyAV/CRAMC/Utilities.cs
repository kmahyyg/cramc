using CRAMC.Common;
using System.IO;
using Serilog;

namespace CRAMC;

public static class Utilities {
    public const FileAttributes FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS = (FileAttributes)0x00400000;
    public const FileAttributes FILE_ATTRIBUTE_UNPINNED = (FileAttributes)0x00100000;
    
    public static long CheckFileSizeOnLocalDisk(string path) {
        // if successful, return file size, otherwise return -1
        //
        // for windows:
        // file on-demand, size on disk == 0, and with:
        // Windows File Attr: O FILE_ATTRIBUTE_OFFLINE (0x00001000)
        // Windows File Attr: U FILE_ATTRIBUTE_UNPINNED (0x00100000)
        // https://ss64.com/nt/attrib.html 
        // Windows File Attr: M FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS (0x00400000)
        //
        // for macos/linux:
        // directly return file.exists and fileinfo.size
        if (File.Exists(path)) {
            if (RuntimeOpts.IsWindows) {
                var fAttr = File.GetAttributes(path);
                if (fAttr.HasFlag(FILE_ATTRIBUTE_UNPINNED) || fAttr.HasFlag(FileAttributes.Offline) ||
                    fAttr.HasFlag(FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS)) {
                    // file stored on remote disk, not available on local
                    return -1;
                }
            }
            else {
                Log.Warning("Detected non-Windows environment, returned size is logical size, not size on disk.");
            }
            var fInfo = new FileInfo(path);
            return fInfo.Length;
        }
        return -1;
    }
}