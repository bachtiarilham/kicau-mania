package model

type RestreamView struct {
	Target  string `json:"target"`
	Channel string `json:"channel_id"`
	State   string `json:"state"`
}
