package model

type Agent struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Clients []string `json:"clients"`
	Port    int      `json:"port"`
	Domain  string   `json:"domain"`
	Status  string   `json:"status"`
	AvatarImg  string `json:"avatar_img"`
    CoverImg   string `json:"cover_img"`
    VoiceModel string `json:"voice_model"`
}

type AgentResponse struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Clients []string `json:"clients"`
	Status  string   `json:"status"`
	AvatarImg  string `json:"avatar_img"`
    CoverImg   string `json:"cover_img"`
    VoiceModel string `json:"voice_model"`
}

type CharacterFile struct {
	Name string `json:"name"`
}
