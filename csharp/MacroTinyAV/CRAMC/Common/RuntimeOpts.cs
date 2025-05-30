namespace CRAMC.Common;

public static class RuntimeOpts {
    public static bool NoPrivilegedActions { get; set; }
    public static bool DryRun { get; set; }
    public static bool DoNotScanDisk { get; set; }
    public static bool IsWindows { get; set; }
    public static bool TryHardening { get; set; }
    public static string ActionPath { get; set; }
}