package peer_stats

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/NetSepio/erebrus/api/v1/service/util"
	"github.com/NetSepio/erebrus/core"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// ApplyRoutes applies router to gin Router
func ApplyRoutes(r *gin.RouterGroup) {
	g := r.Group("/peer")
	{
		g.GET("/stats", readPeerStats)
		g.GET("/bandwidth", readBandwidth)
	}
}

// PeerStats stores the peer's transfer and handshake info
type PeerStats struct {
	PeerID        string `json:"peer_id"`
	SentBytes     string `json:"sent,omitempty"`
	ReceivedBytes string `json:"received,omitempty"`
	LastHandshake string `json:"last_handshake,omitempty"`
}

func readPeerStats(c *gin.Context) {
	peerID := c.Query("peer_id")
	peerData, err := getPeerInfos(peerID)
	if err != nil {
		log.WithFields(util.StandardFields).Error("Failure in reading server")
		response := core.MakeErrorResponse(500, err.Error(), nil, nil, nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": peerData,
	})
}

func readBandwidth(c *gin.Context) {
	peerData, err := getBandwidthStats()
	if err != nil {
		log.WithFields(util.StandardFields).Error("Failure in reading server")
		response := core.MakeErrorResponse(500, err.Error(), nil, nil, nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": peerData,
	})
}

// // Function to get stats for all peers and output them as JSON
// func getPeerStatsJSON() (string, error) {
// 	// Run the `wg show` command to get peer stats
// 	cmd := exec.Command("wg", "show", "all")
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
// 	err := cmd.Run()
// 	if err != nil {
// 		return "", err
// 	}

// 	lines := strings.Split(out.String(), "\n")
// 	peerStats := []PeerStats{}

// 	// Regex patterns to capture the peer info
// 	peerIDRegex := regexp.MustCompile(`peer:\s+(\S+)`)
// 	transferRegex := regexp.MustCompile(`transfer:\s+(\d+)\s+bytes received,\s+(\d+)\s+bytes sent`)
// 	handshakeRegex := regexp.MustCompile(`latest handshake:\s+(\d+)`)

// 	var currentPeer string
// 	for _, line := range lines {
// 		line = strings.TrimSpace(line)

// 		// Detect a new peer ID
// 		if peerIDRegex.MatchString(line) {
// 			matches := peerIDRegex.FindStringSubmatch(line)
// 			currentPeer = matches[1]
// 			peerStats = append(peerStats, PeerStats{PeerID: currentPeer})
// 		}

// 		// Capture the transfer data for the current peer
// 		if transferRegex.MatchString(line) && currentPeer != "" {
// 			matches := transferRegex.FindStringSubmatch(line)
// 			for i := range peerStats {
// 				if peerStats[i].PeerID == currentPeer {
// 					peerStats[i].ReceivedBytes = matches[1]
// 					peerStats[i].SentBytes = matches[2]
// 					break
// 				}
// 			}
// 		}

// 		// Capture the latest handshake for the current peer
// 		if handshakeRegex.MatchString(line) && currentPeer != "" {
// 			matches := handshakeRegex.FindStringSubmatch(line)
// 			for i := range peerStats {
// 				if peerStats[i].PeerID == currentPeer {
// 					peerStats[i].LastHandshake = matches[1]
// 					break
// 				}
// 			}
// 		}
// 	}

// 	// Filter out peers that don't have any relevant data
// 	var filteredStats []PeerStats
// 	for _, stats := range peerStats {
// 		if stats.SentBytes != "" || stats.ReceivedBytes != "" || stats.LastHandshake != "" {
// 			filteredStats = append(filteredStats, stats)
// 		}
// 	}

// 	// Convert filtered stats to JSON format
// 	jsonOutput, err := json.MarshalIndent(filteredStats, "", "  ")
// 	if err != nil {
// 		return "", err
// 	}

// 	return string(jsonOutput), nil
// }

func getPeerInfo(peerID string) (string, error) {
	cmd := exec.Command("wg", "show", "all")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error executing command: %v", err)
	}

	// print the whole output
	fmt.Println(string(output))

	var result strings.Builder
	found := false
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, peerID) {
			result.WriteString(line + "\n")
			found = true
			continue
		}

		if found {
			if strings.TrimSpace(line) == "" {
				break
			}
			result.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading output: %v", err)
	}

	if result.Len() == 0 {
		return "", fmt.Errorf("peer not found")
	}

	return result.String(), nil
}

// func handler(c *gin.Context) {
// 	peerID := c.Query("peerID")
// 	if peerID == "" {
// 		c.JSON(400, gin.H{"error": "Missing peerID parameter"})
// 		return
// 	}

// 	info, err := getPeerInfo(peerID)
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.String(200, info)
// }

// wg show all | awk '/peer: xUNUuzfDfxlMWu4ZPSIym6jxTT16b86SQV+8lSYcjmc=/ {print; f=1; next} f && NF==0 {f=0} f'

type PeerInfo struct {
	Peer            string
	PresharedKey    string
	Endpoint        string
	AllowedIPs      string
	LatestHandshake string
	Transfer        string
}

func parsePeerInfo(output string) (PeerInfo, error) {
	if output == "" {
		return PeerInfo{}, errors.New("invalid peer ID or no data found")
	}

	lines := strings.Split(output, "\n")
	var info PeerInfo

	for _, line := range lines {
		if strings.HasPrefix(line, "peer:") {
			info.Peer = strings.TrimSpace(strings.TrimPrefix(line, "peer:"))
		} else if strings.Contains(line, "preshared key:") {
			info.PresharedKey = strings.TrimSpace(strings.TrimPrefix(line, "preshared key:"))
		} else if strings.Contains(line, "endpoint:") {
			info.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
		} else if strings.Contains(line, "allowed ips:") {
			info.AllowedIPs = strings.TrimSpace(strings.TrimPrefix(line, "allowed ips:"))
		} else if strings.Contains(line, "latest handshake:") {
			info.LatestHandshake = strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
		} else if strings.Contains(line, "transfer:") {
			info.Transfer = strings.TrimSpace(strings.TrimPrefix(line, "transfer:"))
		}
	}

	// Ensure default values for missing fields
	if info.Endpoint == "" {
		info.Endpoint = "N/A"
	}
	if info.LatestHandshake == "" {
		info.LatestHandshake = "N/A"
	}
	if info.Transfer == "" {
		info.Transfer = "N/A"
	}

	return info, nil
}

func getPeerInfos(peerID string) (PeerInfo, error) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("wg show all | awk '/peer: %s/ {print; f=1; next} f && NF==0 {f=0} f'", peerID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return PeerInfo{}, errors.New("failed to execute command or invalid peer ID")
	}

	return parsePeerInfo(strings.TrimSpace(string(output)))
}
