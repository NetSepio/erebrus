package caddy

type ServicePayload struct {
	Name      string `json:"name" binding:"required"`
	IPAddress string `json:"ipAddress" binding:"required"`
	Port      string `json:"port" binding:"required"`
}
