package model

type Filter struct {
	Search, Bird                                 string
	Arena, Cursor, Event, Round, Owner, Role, EO string
	Limit                                        int
}
type Score struct {
	CriteriaID string `json:"criteria_id"`
	Value      string `json:"score_value"`
}
