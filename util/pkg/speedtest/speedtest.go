package speedtest

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

func GetSpeedtestResults() (downloadSpeed, uploadSpeed float64, err error) {
	cmd := exec.Command("speedtest", "--json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to execute 'speedtest --json': %v", err)
	}

	// Parse the Speedtest results
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return 0, 0, fmt.Errorf("failed to parse Speedtest results: %v", err)
	}

	downloadSpeed, ok := result["download"].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("download speed not found in Speedtest results")
	}

	uploadSpeed, ok = result["upload"].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("upload speed not found in Speedtest results")
	}

	return downloadSpeed, uploadSpeed, nil
}
