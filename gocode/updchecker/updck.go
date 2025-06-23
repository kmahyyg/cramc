package updchecker

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

const (
	updCheckerUrl = "https://github.com/kmahyyg/cramc/raw/refs/heads/v4_yarax/assets/latest_version.json"
)

type LatestVersion struct {
	DatabaseVersion int `json:"databaseVersion"`
	ProgramRevision int `json:"programRevision"`
}

func CheckUpdateFromInternet() (*LatestVersion, error) {
	hClient := &http.Client{
		Transport: &http.Transport{},
	}
	hClient.Timeout = time.Second * 5
	req, err := http.NewRequest("GET", updCheckerUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36 Go-CRAMC-UpdateChecker/1.0")
	resp, err := hClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	latestInfo := &LatestVersion{}
	latestJsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(latestJsonBytes, latestInfo)
	if err != nil {
		return nil, err
	}
	return latestInfo, nil
}
