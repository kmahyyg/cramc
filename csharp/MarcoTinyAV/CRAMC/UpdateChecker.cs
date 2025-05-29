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
        try {
            using (var httpClient = new HttpClient()) {
                // Set a user agent to avoid potential blocking
                httpClient.DefaultRequestHeaders.Add("User-Agent", "CRAMC-UpdateChecker/1.0");
                httpClient.Timeout = new TimeSpan(0, 0, 5);
                
                Log.Debug("Fetching latest version from: {Url}", _versionCheckUrl);
                
                var response = await httpClient.GetAsync(_versionCheckUrl);
                response.EnsureSuccessStatusCode();
                
                var jsonContent = await response.Content.ReadAsStringAsync();
                Log.Debug("Received version data: {JsonContent}", jsonContent);
                
                var latestVersion = JsonConvert.DeserializeObject<LatestVersion>(jsonContent);
                
                if (latestVersion == null) {
                    Log.Error("Failed to deserialize version information");
                    throw new InvalidOperationException("Failed to parse version information from GitHub");
                }
                
                _latestVersion = latestVersion;
                Log.Information("Successfully retrieved latest version - Database: {DbVersion}, Program: {ProgVersion}",
                    latestVersion.databaseVersion, latestVersion.programRevision);
                
                return latestVersion;
            }
        }
        catch (HttpRequestException ex) {
            Log.Error(ex, "Network error while fetching version information");
            throw new InvalidOperationException("Failed to fetch version information due to network error", ex);
        }
        catch (JsonException ex) {
            Log.Error(ex, "Failed to parse version information JSON");
            throw new InvalidOperationException("Failed to parse version information", ex);
        }
        catch (Exception ex) {
            Log.Error(ex, "Unexpected error while fetching version information");
            throw;
        }
    }
    
    public int CheckProgramUpdates() {
        if (_latestVersion == null) {
            try {
                GetLatestVersionFromGitHub().GetAwaiter().GetResult();
            }
            catch (Exception ex) {
                Log.Error(ex, "Failed to fetch latest version information");
                return -1; // Indicates error/unavailable
            }
        }
        return _latestVersion!.programRevision;
    }

    public int CheckDatabaseUpdates() {
        if (_latestVersion == null) {
            try {
                GetLatestVersionFromGitHub().GetAwaiter().GetResult();
            }
            catch (Exception ex) {
                Log.Error(ex, "Failed to fetch latest version information");
                return -1; // Indicates error/unavailable
            }
        }
        return _latestVersion!.databaseVersion;
    }
}