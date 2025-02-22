package peer_stats

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// ClientStats represents the bandwidth statistics of a WireGuard client
type ClientStats struct {
	Client string `json:"client"`
	RX     string `json:"rx"`
	TX     string `json:"tx"`
}

// getBandwidthStats fetches the bandwidth stats of WireGuard clients
func getBandwidthStats() ([]ClientStats, error) {
	var clients []ClientStats

	// Get the latest handshakes
	cmd := exec.Command("bash", "-c", "wg show wg0 latest-handshakes")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	nowCmd := exec.Command("date", "+%s")
	nowOut, err := nowCmd.Output()
	if err != nil {
		return nil, err
	}

	now, err := strconv.Atoi(strings.TrimSpace(string(nowOut)))
	if err != nil {
		return nil, err
	}

	var activeClients []string
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		handshakeTime, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		if handshakeTime > 0 && (now-handshakeTime) < 120 {
			activeClients = append(activeClients, fields[0])
		}
	}

	// Get the transfer stats
	cmd = exec.Command("wg", "show", "wg0", "transfer")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	transferStats := out.String()
	for _, client := range activeClients {
		for _, line := range strings.Split(transferStats, "\n") {
			if strings.Contains(line, client) {
				fields := strings.Fields(line)
				if len(fields) < 3 {
					continue
				}

				rxBytes, err := strconv.ParseFloat(fields[1], 64)
				if err != nil {
					continue
				}
				txBytes, err := strconv.ParseFloat(fields[2], 64)
				if err != nil {
					continue
				}

				rxMB := rxBytes / 1024 / 1024
				txMB := txBytes / 1024 / 1024

				clients = append(clients, ClientStats{
					Client: client,
					RX:     strconv.FormatFloat(rxMB, 'f', 4, 64) + " MB",
					TX:     strconv.FormatFloat(txMB, 'f', 4, 64) + " MB",
				})
			}
		}
	}

	return clients, nil
}
