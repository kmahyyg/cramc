using Newtonsoft.Json;

namespace CRAMC_Office;

public class StatusResponse {
    [JsonProperty("status")]
    public string Status;
    
    [JsonProperty("filesPendingInQueue")]
    public int FilesPendingInQueue;
}

public class SingleDocToBeSanitized {
    [JsonProperty("path")]
    public string Path;
    
    [JsonProperty("action")]
    public string Action;
    
    [JsonProperty("detectionName")]
    public string DetectionName;
    
    [JsonProperty("module")]
    public string DestModule;
}

public class DocsToBeSanitized {
    [JsonProperty("counter")]
    public int Counter;
    
    [JsonProperty("toProcess")]
    public List<SingleDocToBeSanitized> ToProcess;
}

public class SingleSanitizedDocResp : SingleDocToBeSanitized {
    [JsonProperty("isSuccess")]
    public bool IsSuccess { get; set; }
    
    [JsonProperty("additionalMsg")]
    public string AdditionalMsg { get; set; }
    
    public SingleSanitizedDocResp(SingleDocToBeSanitized source)
    {
        Path = source.Path;
        Action = source.Action;
        DetectionName = source.DetectionName;
        DestModule = source.DestModule;
    }

    public SingleSanitizedDocResp() { }

}


public class SanitizedDocsResp {
    [JsonProperty("counter")]
    public int Counter;
    
    [JsonProperty("processed")]
    public List<SingleSanitizedDocResp> Processed;
}

