package models

import (
	"log"
	"strconv"
	"strings"

	excelAdapterModels "github.com/HyperloopUPV-H8/Backend-H8/excel_adapter/models"
)

type OrderData map[string]OrderDescription

func isNumeric(kind string) bool {
	return (kind == "uint8" ||
		kind == "uint16" ||
		kind == "uint32" ||
		kind == "uint64" ||
		kind == "int8" ||
		kind == "int16" ||
		kind == "int32" ||
		kind == "int64" ||
		kind == "float32" ||
		kind == "float64")
}

func (orderData *OrderData) AddPacket(globalInfo excelAdapterModels.GlobalInfo, board string, ip string, desc excelAdapterModels.Description, values []excelAdapterModels.Value) {
	if desc.Type != "order" {
		return
	}

	id, err := strconv.ParseUint(desc.ID, 10, 16)
	if err != nil {
		log.Fatalf("order transfer: AddPacket: %s\n", err)
	}

	fields := make(map[string]Field, len(values))
	for _, value := range values {
		fields[value.Name] = getField(value.Name, value.Type)
	}

	(*orderData)[desc.Name] = OrderDescription{
		ID:     uint16(id),
		Name:   desc.Name,
		Fields: fields,
	}
}

func getField(name string, valueType string) Field {
	if isNumeric(valueType) {
		return Field{
			Name: name,
			ValueType: Value{
				Kind:  "numeric",
				Value: valueType,
			},
		}
	} else if valueType == "bool" {
		return Field{
			Name: name,
			ValueType: Value{
				Kind:  "boolean",
				Value: "",
			},
		}
	} else {
		return Field{
			Name: name,
			ValueType: Value{
				Kind:  "enum",
				Value: getEnumMembers(valueType),
			},
		}
	}
}

func getEnumMembers(enumExp string) []string {
	trimmedEnumExp := strings.Replace(enumExp, " ", "", -1)
	firstParenthesisIndex := strings.Index(trimmedEnumExp, "(")
	lastParenthesisIndex := strings.LastIndex(trimmedEnumExp, ")")

	return strings.Split(trimmedEnumExp[firstParenthesisIndex+1:lastParenthesisIndex], ",")
}

type OrderDescription struct {
	ID     uint16           `json:"id"`
	Name   string           `json:"name"`
	Fields map[string]Field `json:"fields"`
}

type Field struct {
	Name      string `json:"name"`
	ValueType Value  `json:"valueType"`
}

type Value struct {
	Kind  string `json:"kind"`
	Value any    `json:"value"`
}
