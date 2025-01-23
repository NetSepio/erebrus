package model

type Agent struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Clients []string `json:"clients"`
	Port    int      `json:"port"`
	Domain  string   `json:"domain"`
	Status  string   `json:"status"`
}

type AgentResponse struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Clients []string `json:"clients"`
	Status  string   `json:"status"`
}

type CharacterFile struct {
	Name string `json:"name"`
}
