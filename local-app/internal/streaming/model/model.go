package model

type RestreamView struct {
	Target  string `json:"target"`
	Channel string `json:"channel_id"`
	State   string `json:"state"`
}

type StartRequest struct {
	Channel string `json:"channel_id" binding:"required"`
	Target  string `json:"target" binding:"required"`
}
