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
        // Windows File Attr: LM FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS (0x00400000)
        //
        // for macos/linux:
        // directly return file.exists and fileinfo.size
        if (File.Exists(path)) {
            if (RuntimeOpts.IsWindows)
            {
                
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