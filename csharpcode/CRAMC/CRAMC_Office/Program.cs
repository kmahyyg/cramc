using System;
using System.IO;
using System.Net.Http;

namespace CRAMC_Office;

class Program {
    static void Main(string[] args) {
        SentrySdk.Init(opts => {
            opts.Dsn = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872";
            opts.AutoSessionTracking = true;
            opts.SendDefaultPii = true;
            opts.AttachStacktrace = true;
        });
        Console.WriteLine("Hello, World!");
    }
}