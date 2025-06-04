using System.Net.Http;
using Newtonsoft.Json;
using System.Net;
using System.Net.Http;
using Serilog;

namespace CRAMC_Office;

public class CommOverRPC {
    private readonly HttpClient _Client = new HttpClient() {
        Timeout = TimeSpan.FromSeconds(3),
    };
    public string _RPCServerAddr;
    public string _OneTimeSecret;
    private string getStatusUrl;
    private string fetchPendingUrl;
    private string reportSanitizedUrl;

    public CommOverRPC(string Addr, string Secret) {
        _Client.DefaultRequestHeaders.UserAgent.ParseAdd("CRAMC-Office-Sanitizer/1.0");
        _RPCServerAddr = Addr;
        _OneTimeSecret = Secret;
        
        getStatusUrl = $"http://{_RPCServerAddr}/{_OneTimeSecret}/getStatus";
        fetchPendingUrl = $"http://{_RPCServerAddr}/{_OneTimeSecret}/pendingFiles";
        reportSanitizedUrl = $"http://{_RPCServerAddr}/{_OneTimeSecret}/fileHandled";
    }

    public async Task<StatusResponse?> RetrieveRPCServerStatus() {
        try {
            var resp = await _Client.GetAsync(getStatusUrl);
            if (resp.StatusCode == HttpStatusCode.OK) {
                var content = await resp.Content.ReadAsStringAsync();
                var statusResp = JsonConvert.DeserializeObject<StatusResponse>(content);
                if (statusResp != null) {
                    return statusResp;
                } 
            }
        } catch (Exception e) {
            Log.Error(e, "Error when retrieving RPC Server status");
        }
        return null;
    }
    
    public async Task<DocsToBeSanitized?> RetrievePendingDocs() {
        try {
            var resp = await _Client.GetAsync(fetchPendingUrl);
            if (resp.StatusCode == HttpStatusCode.OK) {
                var content = await resp.Content.ReadAsStringAsync();
                var docsResp = JsonConvert.DeserializeObject<DocsToBeSanitized>(content);
                if (docsResp != null) {
                    return docsResp;
                } 
            }

            if (resp.StatusCode == HttpStatusCode.NoContent) {
                Log.Information("RPC server returned no pending files.");
                return null;
            }
        } catch (Exception e) {
            Log.Error(e, "Error when retrieving pending docs");
        }
        return null;
    }

    public async Task ReportSanitized(SanitizedDocsResp docsResp) {
        // convert docsResp to json and post to reportSanitizedUrl
        try {
            var jsonRespStr = JsonConvert.SerializeObject(docsResp);
            var content = new StringContent(jsonRespStr, System.Text.Encoding.UTF8, "application/json");
        
            var resp = await _Client.PostAsync(reportSanitizedUrl, content);
            if (resp.StatusCode != HttpStatusCode.OK) {
                Log.Warning("Failed to report sanitized docs. Status code: {StatusCode}", resp.StatusCode);
            }
            else {
                Log.Fatal("Failed to report sanitized docs: {err}", resp.Content.ToString());
            }
        } catch (Exception e) {
            Log.Error(e, "Error when reporting sanitized docs");
        }
    }
}