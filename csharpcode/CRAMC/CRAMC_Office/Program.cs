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

        while (true) {
            Thread.Sleep(5000);
            //TODO: request 
            
        }
    }
}