using System;
using Serilog;
using System.Net;
using System.Net.Http;
using System.Threading.Tasks;
using Newtonsoft.Json;

namespace CRAMC;

internal class UpdateChecker {
    private static string _versionCheckUrl =
        "https://cdn.jsdelivr.net/gh/kmahyyg/cramc@master/assets/latest_version.json";

    private static int _currentProgramRev = 1;

    public class LatestVersion {
        [JsonProperty]
        public int databaseVersion { get; set; }
        [JsonProperty]
        public int programRevision { get; set; }
    }

    private LatestVersion? _latestVersion;

    public async Task<LatestVersion> GetLatestVersionFromGitHub() {
    }
    
    public int CheckProgramUpdates() {
        if (_latestVersion == null) {

        } else {
            return _latestVersion.databaseVersion;
        }
    }

    public int CheckDatabaseUpdates() {
        
    }
}