package types

import "encoding/json"

type WSMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}
