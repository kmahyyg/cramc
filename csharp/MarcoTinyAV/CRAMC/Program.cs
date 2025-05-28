using Sentry;
using System;
using System.IO;
using System.Collections.Generic;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using CommandLine;
using Serilog;

namespace CRAMC;

class Program
{
    public class ProcOptions
    {
        [Option("notAdmin", Default = false, HelpText = "Assumes current user is unprivileged. This generally skips operation that requires admin privileges and prevent from reading MFT.")]
        public bool NotAdmin { get; set; }
        
        [Option('a', "actionPath", Default = "C:\\Users", HelpText = "The path to the files you want to scan. To balance scanning speed and false positive rate, we recommend to scan User profile only. In case there're multiple folders, use | as separator. By default, we use recursive search.")]
        public string ActionPath { get; set; }
        
        [Option('f', "forceAction", Default = true, HelpText = "Force action to be taken regardless current circumstances.")]
        public bool ForceAction { get; set; }
        
        [Option("yaraRuleDir", Default = "yrules/", HelpText = "The directory where compiled yara rules are located.")]
        public string YaraRuleDir { get; set; }
        
        [Option("ignoreRemoteFile", Default = true, HelpText = "Ignore remote files that are not on disk.")]
        public bool IgnoreRemoteFile { get; set; }
        
        [Option('e',"enableHardening", Default = true, HelpText = "Enables hardening measure to prevent further infection.")]
        public bool EnableHardening { get; set; }
        
        [Option('b', "backupFileDir", Default = "fbak/", HelpText = "The directory where original files are saved.")]
        public string BackupFileDir { get; set; }
        
        [Option("alwaysRename", Default = true, HelpText = "Always rename remediated file, this is used to prevent server-side state cache in case of file in cloud storage.")]
        public bool AlwaysRename { get; set; }
        
        [Option("cleanupDB", Default = "cramc_db.json", HelpText = "Cleanup DB location. If you don't know its meaning, do not touch this option.")]
        public string CleanupDBLocation { get; set; }
        
        [Option("logFile", Default = "cramc.log", HelpText = "Log file location.")]
        public string LogFile { get; set; }
    }
    
    static void Main(string[] args)
    {
        // initialize sentry.io sdk for error analysis and APM
        SentrySdk.Init(opts =>
        {
            opts.Dsn = "https://af1658f8654e2f490466ef093b2d6b7f@o132236.ingest.us.sentry.io/4509401173327872";
            opts.AutoSessionTracking = true;
        });
        // initialize logger provided by Serilog
        
        // dealing with operation called by user
        
    }
}