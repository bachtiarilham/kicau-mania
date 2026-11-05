package media

// Status is process health only; it does not assert every camera is available.
type Status struct {
	State string `json:"state"`
}
