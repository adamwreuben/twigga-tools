package models

type Config struct {
	Status         bool   `json:"status"`
	BaseURL        string `json:"baseURL"`
	AccountBaseURL string `json:"accountBaseURL"`
	ProjectId      string `json:"projectId"`
	Token          string `json:"token"`
}
