package common

type CRAMCCleanupDB struct {
	Version   int64                  `json:"version"`
	Solutions []*SingleVirusSolution `json:"solutions"`
}

type SingleVirusSolution struct {
	Name                string                 `json:"name"`
	DestModule          string                 `json:"module"`
	Action              string                 `json:"action"`
	MustHarden          bool                   `json:"mustHarden"`
	AllowRepeatedHarden bool                   `json:"allowRepeatedHarden"`
	HardenMeasures      []*SingleHardenMeasure `json:"hardenMeasures"`
}

type SingleHardenMeasure struct {
	Action string `json:"action"`
	Type   string `json:"type"`
	Dest   string `json:"dest"`
}
