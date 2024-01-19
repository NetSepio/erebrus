package stats

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetWireGuardStats() (map[string]map[string]int64, error) {
	cmd := exec.Command("wg", "show", "all", "transfer")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'sudo wg show all transfer': %v", err)
	}

	stats := make(map[string]map[string]int64)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			peerPublicKey := fields[1]
			receivedBytes := fields[2]
			transmittedBytes := fields[3]

			if _, ok := stats[peerPublicKey]; !ok {
				stats[peerPublicKey] = make(map[string]int64)
			}

			stats[peerPublicKey]["ReceivedBytes"] = parseBytes(receivedBytes)
			stats[peerPublicKey]["TransmittedBytes"] = parseBytes(transmittedBytes)
		}
	}

	return stats, nil
}

type WireGuardStats struct {
	Interface        string
	PeerPublicKey    string
	ReceivedBytes    int64
	TransmittedBytes int64
}

func GetWireGuardStatsForPeer(publicKey string) (*WireGuardStats, error) {
	cmd := exec.Command("wg", "show", "all", "transfer")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'sudo wg show all transfer': %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[1] == publicKey {
			return &WireGuardStats{
				Interface:        fields[0],
				PeerPublicKey:    fields[1],
				ReceivedBytes:    parseBytes(fields[2]),
				TransmittedBytes: parseBytes(fields[3]),
			}, nil
		}
	}

	return nil, fmt.Errorf("stats not found for public key: %s", publicKey)
}

func parseBytes(bytesStr string) int64 {
	var unit int64

	fmt.Sscanf(bytesStr, "%d", &unit)

	return unit
}
