package models

import (
	"log"
	"strconv"
	"strings"

	excelAdapterModels "github.com/HyperloopUPV-H8/Backend-H8/excel_adapter/models"
)

type OrderData map[string]OrderDescription

func NewOrderData() *OrderData {
	return &OrderData{}
}

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

func (orderData *OrderData) AddGlobal(global excelAdapterModels.GlobalInfo) {}

func (orderData *OrderData) AddPacket(boardName string, packet excelAdapterModels.Packet) {
	if packet.Description.Type != "order" {
		return
	}

	id, err := strconv.ParseUint(packet.Description.ID, 10, 16)
	if err != nil {
		log.Fatalf("order transfer: AddPacket: %s\n", err)
	}

	fields := make(map[string]FieldDescription, len(packet.Values))
	for _, value := range packet.Values {
		fields[value.Name] = getField(value.ID, value.Type)
	}

	(*orderData)[packet.Description.Name] = OrderDescription{
		ID:     uint16(id),
		Name:   packet.Description.Name,
		Fields: fields,
	}
}

func getField(name string, valueType string) FieldDescription {
	if isNumeric(valueType) {
		return FieldDescription{
			Name: name,
			ValueType: Value{
				Kind:  "numeric",
				Value: valueType,
			},
		}
	} else if valueType == "bool" {
		return FieldDescription{
			Name: name,
			ValueType: Value{
				Kind:  "boolean",
				Value: "",
			},
		}
	} else {
		return FieldDescription{
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
	ID     uint16                      `json:"id"`
	Name   string                      `json:"name"`
	Fields map[string]FieldDescription `json:"fields"`
}

type FieldDescription struct {
	Name      string `json:"name"`
	ValueType Value  `json:"valueType"`
}

type Value struct {
	Kind  string `json:"kind"`
	Value any    `json:"value"`
}
