package utils

import (
	"encoding/binary"
	"encoding/json"
	"net"
)

const MAGIC_LEN = 7
const HEADER_LEN = 12
const (
	IPC_TYPE_RUN_COMMAND    = 0
	IPC_TYPE_GET_MONITORS   = 1
	IPC_TYPE_GET_TAGS       = 2
	IPC_TYPE_GET_LAYOUTS    = 3
	IPC_TYPE_GET_DWM_CLIENT = 4
	IPC_TYPE_SUBSCRIBE      = 5
	IPC_TYPE_EVENT          = 6
)

func GenerateMessage(message []byte, message_type int) []byte {
	header := []byte("DWM-IPC")
	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(message)))
	header = append(header, size...)
	header = append(header, byte(message_type))
	return append(header, message...)
}

func SendStruct(c *net.Conn, msg_struct any, message_type int) error {
	message, _ := json.Marshal(msg_struct)
	_, err := (*c).Write(GenerateMessage(message, message_type))
	return err
}

type IPCSubscribePayload struct {
	Event  string `json:"event"`
	Action string `json:"action"`
}

type IPCGetDWMClientPayload struct {
	Id int `json:"client_window_id"`
}

type TagChangeEventState struct {
	Selected int `json:"selected"`
	Occupied int `json:"occupied"`
	Urgent   int `json:"urgent"`
}

type IPCTagChangeEvent struct {
	Event struct {
		MonitorNumber int                 `json:"monitor_number"`
		OldState      TagChangeEventState `json:"old_state"`
		NewState      TagChangeEventState `json:"new_state"`
	} `json:"tag_change_event"`
}

type IPCLayoutChangeEvent struct {
	Event struct {
		MonitorNumber int    `json:"monitor_number"`
		NewSymbol     string `json:"new_symbol"`
	} `json:"layout_change_event"`
}

type Tag struct {
	BitMask    int    `json:"bit_mask"`
	Name       string `json:"name"`
	IsUrgent   bool
	IsActive   bool
	IsOccupied bool
}

type Monitor struct {
	Number     int                 `json:"num"`
	IsSelected bool                `json:"is_selected"`
	TagState   TagChangeEventState `json:"tag_state"`
	Layout     struct {
		Symbol struct {
			Current string `json:"current"`
		} `json:"symbol"`
	} `json:"layout"`
	Clients struct {
		Selected int `json:"selected"`
		// Stack []int `json:"stack"`
		// All []int `json:"all"`
	}
}

type Client struct {
	Name string `json:"name"`
}
