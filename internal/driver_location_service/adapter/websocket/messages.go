package ws

type Message struct {
	Type  string      `json:"type"`
	Data  interface{} `json:"data,omitempty"`
	Token string      `json:"token,omitempty"` // only for auth
}

const (
	MsgTypeAuth     = "auth"
	MsgRideOffer    = "ride_offer"
	MsgRideDetails  = "ride_details"
	MsgRideResponse = "ride_response"
)
