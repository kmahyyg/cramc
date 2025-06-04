using System;
using System.IO;
using System.Net.Http;
using Serilog;
using Sentry;

namespace CRAMC_Office;

class Program {
    private static readonly string _runtimeLogFile = "cramc_cleaner_csharp.log";
    static void Main(string[] args) {
        Log.Logger = new LoggerConfiguration()
            .MinimumLevel.Information()
            .WriteTo.Console()
            .WriteTo.File(_runtimeLogFile)
            .CreateLogger();
        if (args.Length != 3)
        {
            Log.Fatal("Usage: cramc_o365_cleaner.exe <RPC Server Addr> <One-Time Secret>");
            Environment.Exit(1);
        }
        SentrySdk.Init(opts => {
            opts.Dsn = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872";
            opts.AutoSessionTracking = true;
            opts.SendDefaultPii = true;
            opts.AttachStacktrace = true;
        });
        Log.Information("Start CRAMC Office.");
        Log.Information("RPC Server Addr: {RPCServerAddr}", args[1]);
        Log.Information("One-Time Secret: {OneTimeSecret}", args[2]);
        CommOverRPC rpcComm = new CommOverRPC(args[1], args[2]);
        var failedCount = 0;
        while (true) {
            Thread.Sleep(4000);
            if (failedCount > 3) {
                Log.Fatal("Too many failed attempts. Exiting.");
                Environment.Exit(1);
            }
            try {
                var rpcServStatusResp = rpcComm.RetrieveRPCServerStatus().GetAwaiter().GetResult();
                if (rpcServStatusResp == null) {
                    failedCount += 1;
                    continue;
                }
                if (rpcServStatusResp.Status == "stopped") {
                    Log.Information("RPC Server is stopped. Exiting.");
                    break;
                } else if (rpcServStatusResp.Status == "running") {
                    Log.Information("RPC Server is running. Continuing.");
                    if (rpcServStatusResp.FilesPendingInQueue > 0) {
                        Log.Information("RPC Server has {FilesPendingInQueue} files pending in queue. Continuing.", rpcServStatusResp.FilesPendingInQueue);
                        var docsToBeSanitized = rpcComm.RetrievePendingDocs().GetAwaiter().GetResult();
                        if (docsToBeSanitized == null) {
                            Log.Error("Fetch Pending Docs failed./No files are pending in queue.");
                            continue;
                        }
                        var officeFileOp = new OfficeFileOperator();
                        var officeFileOpResp = new SanitizedDocsResp();
                        officeFileOpResp.Processed = new List<SingleSanitizedDocResp>();
                        if (docsToBeSanitized.Counter > 0) {
                            foreach (var item in docsToBeSanitized.ToProcess) {
                                var ssDocResp = officeFileOp.DispatchAction(item).GetAwaiter().GetResult();
                                if (ssDocResp.IsSuccess) {officeFileOpResp.Counter += 1;}
                                officeFileOpResp.Processed.Add(ssDocResp);
                            }
                        }
                        rpcComm.ReportSanitized(officeFileOpResp).GetAwaiter().GetResult();
                        Log.Information("Current batch process finished.");
                    }
                    else {
                        Log.Information("RPC Server has no files pending in queue.");
                    }
                }
            }
            catch (Exception e) {
                Log.Error(e, "Error when handling RPC tasks. ");
            }
        }
    }
}