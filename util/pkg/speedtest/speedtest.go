package speedtest

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type SpeedtestResult struct {
	DownloadSpeed float64 `json:"downloadSpeed"`
	UploadSpeed   float64 `json:"uploadSpeed"`
}

func GetSpeedtestResults() (res *SpeedtestResult, err error) {
	cmd := exec.Command("speedtest-cli", "--json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'speedtest-cli --json': %v", err)
	}

	// Parse the Speedtest results
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Speedtest results: %v", err)
	}

	downloadSpeed, ok := result["download"].(float64)
	if !ok {
		return nil, fmt.Errorf("download speed not found in Speedtest results")
	}

	uploadSpeed, ok := result["upload"].(float64)
	if !ok {
		return nil, fmt.Errorf("upload speed not found in Speedtest results")
	}
	response := &SpeedtestResult{
		DownloadSpeed: downloadSpeed,
		UploadSpeed:   uploadSpeed,
	}
	return response, nil
}
