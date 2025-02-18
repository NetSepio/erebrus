package peer_stats

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os/exec"
	"regexp"
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
	peerData, err := getPeerStatsJSON()
	if err != nil {
		log.WithFields(util.StandardFields).Error("Failure in reading server")
		response := core.MakeErrorResponse(500, err.Error(), nil, nil, nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}

	c.JSON(http.StatusOK, peerData)
}

// Function to get stats for all peers and output them as JSON
func getPeerStatsJSON() (string, error) {
	// Run the `wg show` command to get peer stats
	cmd := exec.Command("wg", "show", "all")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	lines := strings.Split(out.String(), "\n")
	peerStats := []PeerStats{}

	// Regex patterns to capture the peer info
	peerIDRegex := regexp.MustCompile(`peer:\s+(\S+)`)
	transferRegex := regexp.MustCompile(`transfer:\s+(\d+)\s+bytes received,\s+(\d+)\s+bytes sent`)
	handshakeRegex := regexp.MustCompile(`latest handshake:\s+(\d+)`)

	var currentPeer string
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect a new peer ID
		if peerIDRegex.MatchString(line) {
			matches := peerIDRegex.FindStringSubmatch(line)
			currentPeer = matches[1]
			peerStats = append(peerStats, PeerStats{PeerID: currentPeer})
		}

		// Capture the transfer data for the current peer
		if transferRegex.MatchString(line) && currentPeer != "" {
			matches := transferRegex.FindStringSubmatch(line)
			for i := range peerStats {
				if peerStats[i].PeerID == currentPeer {
					peerStats[i].ReceivedBytes = matches[1]
					peerStats[i].SentBytes = matches[2]
					break
				}
			}
		}

		// Capture the latest handshake for the current peer
		if handshakeRegex.MatchString(line) && currentPeer != "" {
			matches := handshakeRegex.FindStringSubmatch(line)
			for i := range peerStats {
				if peerStats[i].PeerID == currentPeer {
					peerStats[i].LastHandshake = matches[1]
					break
				}
			}
		}
	}

	// Filter out peers that don't have any relevant data
	var filteredStats []PeerStats
	for _, stats := range peerStats {
		if stats.SentBytes != "" || stats.ReceivedBytes != "" || stats.LastHandshake != "" {
			filteredStats = append(filteredStats, stats)
		}
	}

	// Convert filtered stats to JSON format
	jsonOutput, err := json.MarshalIndent(filteredStats, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonOutput), nil
}
