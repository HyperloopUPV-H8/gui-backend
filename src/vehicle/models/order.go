package models

type Order struct {
	ID     uint16           `json:"id"`
	Fields map[string]Field `json:"fields"`
}

type Field struct {
	Value     any  `json:"value"`
	IsEnabled bool `json:"isEnabled"`
}

type EmittedOrder struct {
}
