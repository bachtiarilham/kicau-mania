// Package model defines complete participant assessments and optimistic revisions.
package model

type Value struct {
	CriteriaID string `json:"criteria_id"`
	Value      int    `json:"score_value"`
}
type Submit struct {
	OperationID     string  `json:"operation_id"`
	ExpectedVersion int     `json:"expected_version"`
	Values          []Value `json:"values"`
}
