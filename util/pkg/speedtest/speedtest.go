package speedtest

import (
	"fmt"

	"github.com/showwin/speedtest-go/speedtest"
)

type SpeedtestResult struct {
	Latency       string  `json:"latency"`
	DownloadSpeed float64 `json:"downloadSpeed"`
	UploadSpeed   float64 `json:"uploadSpeed"`
}

func GetSpeedtestResults() (res *SpeedtestResult, err error) {
	var speedtestClient = speedtest.New()

	serverList, _ := speedtestClient.FetchServers()
	targets, _ := serverList.FindServer([]int{})
	var response *SpeedtestResult
	for _, s := range targets {
		// Please make sure your host can access this test server,
		// otherwise you will get an error.
		// It is recommended to replace a server at this time
		s.PingTest(nil)
		s.DownloadTest()
		s.UploadTest()
		fmt.Printf("Latency: %s, Download: %f, Upload: %f\n", s.Latency, s.DLSpeed, s.ULSpeed)
		s.Context.Reset() // reset counter
		response = &SpeedtestResult{
			Latency:       s.Latency.String(),
			DownloadSpeed: s.DLSpeed.Mbps(),
			UploadSpeed:   s.ULSpeed.Mbps(),
		}
	}
	return response, nil
}
