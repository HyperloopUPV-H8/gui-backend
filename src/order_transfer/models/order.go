package models

type Order struct {
	ID     uint16         `json:"id"`
	Values map[string]any `json:"fields"`
}
