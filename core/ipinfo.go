package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type IPInfo struct {
	IP       string `json:"ip"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Location string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

var GlobalIPInfo IPInfo

func GetIPInfo() {
	resp, err := http.Get("https://ipinfo.io")
	if err != nil {
		fmt.Println("Error fetching IP information:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	if err := json.Unmarshal(body, &GlobalIPInfo); err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return
	}

	fmt.Println("IP:", GlobalIPInfo.IP)
	fmt.Println("City:", GlobalIPInfo.City)
	fmt.Println("Region:", GlobalIPInfo.Region)
	fmt.Println("Country:", GlobalIPInfo.Country)
	fmt.Println("Location:", GlobalIPInfo.Location)
	fmt.Println("Organization:", GlobalIPInfo.Org)
	fmt.Println("Postal:", GlobalIPInfo.Postal)
	fmt.Println("Timezone:", GlobalIPInfo.Timezone)
}
