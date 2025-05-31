using System.Threading.Channels;

namespace CRAMC.FileUtils;

public interface IFileSearcher {
    // return iterated and matched file number, if error, return -1
    public long FindMatchedFilesUnderPath(string actionPath, string[] allowedExts, Channel<string> matchFileOutputChan);
}