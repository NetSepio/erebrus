package model

// struct name service
type Service struct { // name app
	Name      string `json:"name"`
	Type      string `json:"type"`
	IpAddress string `json:"ipAddress,omitempty"`
	Port      string `json:"port"`
	Domain    string `json:"domain"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// type name services
type ServicesList struct {
	Services []Service `json:"services"`
}
