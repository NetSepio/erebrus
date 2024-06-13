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
	resp, err := http.Get("https://ipinfo.io/json")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	err = json.Unmarshal(body, &GlobalIPInfo)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("IP: %s\n", GlobalIPInfo.IP)
	fmt.Printf("City: %s\n", GlobalIPInfo.City)
	fmt.Printf("Region: %s\n", GlobalIPInfo.Region)
	fmt.Printf("Country: %s\n", GlobalIPInfo.Country)
	fmt.Printf("Location: %s\n", GlobalIPInfo.Location)
	fmt.Printf("Organization: %s\n", GlobalIPInfo.Org)
	fmt.Printf("Postal: %s\n", GlobalIPInfo.Postal)
	fmt.Printf("Timezone: %s\n", GlobalIPInfo.Timezone)
}
