namespace CRAMC.Common;

public struct DetectionResult {
    public DetectionResult(string rule, string path) {
        DetectedRule = rule;
        Filepath = path;
    }
    
    public string DetectedRule { get; set; }
    public string Filepath { get; set; }

    public override string ToString() => $"Detected {DetectedRule} at {Filepath}";
}

public struct SanitizerResult {
    public DetectionResult Detection { get; init; }
    public string SanitizeAction { get; set; }
    public bool IsSuccess { get; set; }
    public string AdditionalMsg { get; set; }

    public SanitizerResult(DetectionResult baseDet, string action, string addiMsg) {
        Detection = baseDet;
        SanitizeAction = action;
        AdditionalMsg = addiMsg;
    }

    public override string ToString() => $"Sanitize Action Taken: {SanitizeAction} , with message: {AdditionalMsg}, Success: {IsSuccess}";
}

public struct HardenerResult {
    public DetectionResult Detection { get; init; }
    public string HardenAction { get; set; }
    public bool IsSuccess {get; set; }
    public string AdditionalMsg { get; set; }

    public override string ToString() => $"Hardener Action Taken: {HardenAction} , with message: {AdditionalMsg}, Success: {IsSuccess}";
} 

public struct HandlingResult {
    public DetectionResult Detection;
    public SanitizerResult Sanitizer;
    public HardenerResult Hardener;
}