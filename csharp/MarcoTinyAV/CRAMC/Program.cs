using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using CommandLine;
using Sentry;
using Serilog;

namespace CRAMC;

internal class Program {
    private static string _searchMethod = "parseMFT";
    public static bool isWindows = RuntimeInformation.IsOSPlatform(OSPlatform.Windows);
    
    // possibly, ".xlt/.xltm" might also get infected, but I haven't observed any of them in my environment
    // same for other extensions (e.g. ".ppt/.docm/.doc/.dot/.dotm/.ppt/.pptm/.pot/.potm/.pps/.ppsm/.ppa/.ppam")
    public string[] operatingExtensions = { ".xls", ".xlsm", ".xlsb" };
    
    public static void Main(string[] args) {
        // initialize sentry.io sdk for error analysis and APM
        SentrySdk.Init(opts => {
            opts.Dsn = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872";
            opts.AutoSessionTracking = true;
        });
        // parse arguments from cmdline then next
        Parser.Default.ParseArguments<ProcOptions>(args)
            .WithParsed(RunWithOptions)   
            .WithNotParsed(HandleArgParseError);
    }

    private static void RunWithOptions(ProcOptions options) {
        // initialize logger provided by Serilog
        Log.Logger = new LoggerConfiguration()
            .MinimumLevel.Information()
            .WriteTo.Console()
            .WriteTo.File(options.LogFile)
            .CreateLogger();
        // dealing with operation called by user
        // check privilege and runtime platform, unfortunately, due to dependency, only Windows is supported.
        if (options.NotAdmin || !CheckUACElevated()) {
            _searchMethod = "walkthrough";
        }
        // cross-platform handling
        if (!isWindows) {
            _searchMethod = "walkthrough";
            options.EnableHardening = false;
            Log.Warning("Detected non-Windows OS, file search method: walkthrough, no hardening measure will be placed.");
            Log.Warning("Detected non-Windows OS, no guarantee on clean up actions, try in best-effort.");
            if (!options.NoScan) {
                Console.Error.WriteLine("Scanner component only works on Windows. Your platform is not supported.");
                Environment.Exit(2);
            }
        } else {
            // TODO
            // isWindows == true && noScan == false, scan first
            // matched result with callback for further action
        }
        // TODO
        // check output list and proceed further
        // cleanup and sync log info to disk
        Log.CloseAndFlush();
    }

    private static void HandleArgParseError(IEnumerable<Error> errs) {
        Console.Error.WriteLine("Failed to parse arguments");
        foreach (var err in errs) {
            Console.Error.WriteLine(err.ToString());
        }
        Environment.Exit(1);
    }

    private static bool CheckUACElevated() {
        // if elevated, return true.
        if (isWindows) {
            //TODO
        }
        return false;
    }
    
    public class ProcOptions {
        [Option("notAdmin", Default = false,
            HelpText =
                "Assumes current user is unprivileged. This generally skips operation that requires admin privileges and prevent from reading MFT.")]
        public bool NotAdmin { get; set; }

        [Option('a', "actionPath", Default = "C:\\Users",
            HelpText =
                "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. In case there're multiple folders, use | as separator. By default, we use recursive search.")]
        public string ActionPath { get; set; }

        [Option('f', "forceAction", Default = true,
            HelpText = "Force action to be taken regardless current circumstances.")]
        public bool ForceAction { get; set; }

        [Option("yaraRuleDir", Default = "yrules/", HelpText = "The directory where compiled yara rules are located.")]
        public string YaraRuleDir { get; set; }

        [Option("ignoreRemoteFile", Default = true, HelpText = "Ignore remote files that are not on disk.")]
        public bool IgnoreRemoteFile { get; set; }

        [Option('e', "enableHardening", Default = true,
            HelpText = "Enables hardening measure to prevent further infection.")]
        public bool EnableHardening { get; set; }

        [Option('b', "backupFileDir", Default = "fbak/", HelpText = "The directory where original files are saved.")]
        public string BackupFileDir { get; set; }

        [Option("alwaysRename", Default = true,
            HelpText =
                "Always rename remediated file, this is used to prevent server-side state cache in case of file in cloud storage.")]
        public bool AlwaysRename { get; set; }

        [Option("cleanupDB", Default = "cramc_db.json",
            HelpText = "Cleanup DB location. If you don't know its meaning, do not touch this option.")]
        public string CleanupDBLocation { get; set; }

        [Option("logFile", Default = "cramc.log", HelpText = "Log file location.")]
        public string LogFile { get; set; }

        [Option("dryRun", Default = false,
            HelpText = "Scan only, take no action on files, record action to be taken in log.")]
        public bool DryRun { get; set; }
        
        [Option("noScan", Default = false, HelpText = "Do not scan files. If platform is not Windows x86_64, yara won't work, you have to set this to true and then run Yara scanner against our rules and save output to ipt_yrscan.lst (default), then provide cleanOnlyFileList with the output file path. Yara-X scanner is not supported yet.")]
        public bool NoScan { get; set; }
        
        [Option("cleanOnlyFileList", Default = "ipt_yrscan.lst", HelpText = "List of Files to be cleaned. Yara scanner output log filename.")]
        public string CleanOnlyFileList { get; set; }
    }
}