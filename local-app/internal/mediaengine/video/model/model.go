package model

type Channel struct {
	ID   string
	WHEP string
}

type ChannelResponse struct {
	ID    string `json:"id"`
	WHEP  string `json:"whep_url"`
	Audio string `json:"audio_path"`
}
