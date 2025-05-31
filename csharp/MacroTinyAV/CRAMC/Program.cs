using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Security.Principal;
using CommandLine;
using CRAMC.Common;
using CRAMC.FileUtils;
using Sentry;
using Serilog;

namespace CRAMC;

internal class Program {
    private const string _runtimeLogFile = "cramc.log";
    private const string _cleanOnlyFileLst = "ipt_yrscan.lst";
    private const string _yaraRuleDir = "yrules/";
    private const string _cleanupDB = "cramc_db.json";
    private const string _backupDir = "fbak/";

    private const string _sentryDSN =
        "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872";

    private static string _searchMethod = "parseMFT";

    // possibly, ".xlt/.xltm" might also get infected, but I haven't observed any of them in my environment
    // same for other extensions (e.g. ".ppt/.docm/.doc/.dot/.dotm/.ppt/.pptm/.pot/.potm/.pps/.ppsm/.ppa/.ppam")
    public static string[] operatingExtensions = { ".xls", ".xlsm", ".xlsb" };

    public static void Main(string[] args) {
        // initialize sentry.io sdk for error analysis and APM
        SentrySdk.Init(opts => {
            opts.Dsn = _sentryDSN;
            opts.AutoSessionTracking = true;
        });

        // check updates, if there's a newer version, exit. 
        var updck = new UpdateChecker();
        updck.CheckProgramUpdates();

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
            .WriteTo.File(_runtimeLogFile)
            .CreateLogger();
        // initialize runtime options
        RuntimeOpts.DoNotScanDisk = options.NoDiskScan;
        // if no-disk-scan, check existence of clean only file list
        if (options.NoDiskScan) {
            if (FileSizeUtils.CheckFileSizeOnLocalDisk(_cleanOnlyFileLst) <= 0) {
                Log.Fatal("No disk scan set, cannot find clean-only file list.");
                Environment.Exit(1);
            }
        }
        // assign user options to general available location
        RuntimeOpts.DryRun = options.DryRun;
        RuntimeOpts.IsWindows = RuntimeInformation.IsOSPlatform(OSPlatform.Windows);
        if (RuntimeOpts.IsWindows && options.EnableHardening) RuntimeOpts.TryHardening = true;
        RuntimeOpts.ActionPath = options.ActionPath;
        // dealing with operation called by user
        if (!RuntimeOpts.DoNotScanDisk) {
            // start searching, searching output to a buffer/stream

        }
        //TODO
        // cleanup and sync log info to disk
        Log.CloseAndFlush();
    }

    private static void HandleArgParseError(IEnumerable<Error> errs) {
        Console.Error.WriteLine("Failed to parse arguments");
        foreach (var err in errs) Console.Error.WriteLine(err.ToString());
        Environment.Exit(1);
    }

    public class ProcOptions {
        [Option('a', "actionPath", Default = "C:\\Users",
            HelpText =
                "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. By default, we use recursive search.")]
        public string ActionPath { get; set; }

        [Option('e', "enableHardening", Default = true,
            HelpText = "Enables hardening measure to prevent further infection.")]
        public bool EnableHardening { get; set; }

        [Option("dryRun", Default = false,
            HelpText = "Scan only, take no action on files, record action to be taken in log.")]
        public bool DryRun { get; set; }

        [Option("noDiskScan", Default = false,
            HelpText =
                "Do not scan files on disk, but supply file list. If platform is not Windows x86_64, yara won't work, you have to set this to true and then run Yara scanner against our rules and save output to ipt_yrscan.lst. Yara-X scanner is not supported yet.")]
        public bool NoDiskScan { get; set; }

        // always rename file to prevent any type of cache.
        //
        // [Option("alwaysRename", Default = true,
        //     HelpText =
        //         "Always rename remediated file, this is used to prevent server-side state cache in case of file in cloud storage.")]
        // public bool AlwaysRename { get; set; }

        // always take action, regardless of flag set status
        //
        // [Option('f', "forceAction", Default = true,
        //     HelpText = "Force action to be taken regardless current circumstances.")]
        // public bool ForceAction { get; set; }
        
        // always assume user is unprivileged, this is used to directly fallback to walkthrough disk
        // update 2: deprecated as always unprivileged, NTFS-boosted search by utilizing MFT may cause OOM due to unpredicted large MFT file size.
        //
        // [Option("notAdmin", Default = false,
        //     HelpText =
        //         "Assumes current user is unprivileged. This generally skips operation that requires admin privileges and prevent from reading MFT.")]
        // public bool NotAdmin { get; set; }
        
        // always ignore remote file, if file is not on disk, no action could be taken and may cause thread hang.
        //
        // [Option("ignoreRemoteFile", Default = true, HelpText = "Ignore remote files that are not on disk.")]
        // public bool IgnoreRemoteFile { get; set; }

        // internal configuration item, should not expose to end-user, just hardcoded, do not remove, leave for ref.
        //
        // [Option("cleanOnlyFileList", Default = "ipt_yrscan.lst", HelpText = "List of Files to be cleaned. Yara scanner output log filename.")]
        // public string CleanOnlyFileList { get; set; }
        //
        // [Option("logFile", Default = "cramc.log", HelpText = "Log file location.")]
        // public string LogFile { get; set; }
        //
        // [Option("cleanupDB", Default = "cramc_db.json",
        //     HelpText = "Cleanup DB location. If you don't know its meaning, do not touch this option.")]
        // public string CleanupDBLocation { get; set; }
        //
        // [Option('b', "backupFileDir", Default = "fbak/", HelpText = "The directory where original files are saved.")]
        // public string BackupFileDir { get; set; }
        //
        // [Option("yaraRuleDir", Default = "yrules/", HelpText = "The directory where compiled yara rules are located.")]
        // public string YaraRuleDir { get; set; }

    }
}