package signalr

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

type negotiationResponse struct {
	Url                     string
	ConnectionToken         string
	ConnectionId            string
	KeepAliveTimeout        float32
	DisconnectTimeout       float32
	ConnectionTimeout       float32
	TryWebSockets           bool
	ProtocolVersion         string
	TransportConnectTimeout float32
	LogPollDelay            float32
}

type Client struct {
	params negotiationResponse
	socket *websocket.Conn
	nextId int
}

func negotiate(scheme, address string) (negotiationResponse, error) {
	var response negotiationResponse

	var negotiationUrl = url.URL{Scheme: scheme, Host: address, Path: "/signalr/negotiate"}

	client := &http.Client{}

	reply, err := client.Get(negotiationUrl.String())
	if err != nil {
		return response, err
	}

	defer reply.Body.Close()

	if body, err := ioutil.ReadAll(reply.Body); err != nil {
		return response, err
	} else if err := json.Unmarshal(body, &response); err != nil {
		return response, err
	} else {
		return response, nil
	}
}

func connectWebsocket(address string, params negotiationResponse, hubs []string) (*websocket.Conn, error) {
	var connectionData = make([]struct {
		Name string `json:"Name"`
	}, len(hubs))
	for i, h := range hubs {
		connectionData[i].Name = h
	}
	connectionDataBytes, err := json.Marshal(connectionData)
	if err != nil {
		return nil, err
	}

	var connectionParameters = url.Values{}
	connectionParameters.Set("transport", "webSockets")
	connectionParameters.Set("clientProtocol", "1.5")
	connectionParameters.Set("connectionToken", params.ConnectionToken)
	connectionParameters.Set("connectionData", string(connectionDataBytes))

	var connectionUrl = url.URL{Scheme: "wss", Host: address, Path: "signalr/connect"}
	connectionUrl.RawQuery = connectionParameters.Encode()

	if conn, _, err := websocket.DefaultDialer.Dial(connectionUrl.String(), nil); err != nil {
		return nil, err
	} else {
		return conn, nil
	}
}

func (self *Client) Read() ([]byte, error) {
	_, data, err := self.socket.ReadMessage()
	return data, err
}

func (self *Client) CallHub(hub, method string, params ...interface{}) error {
	var request = struct {
		Hub        string        `json:"H"`
		Method     string        `json:"M"`
		Arguments  []interface{} `json:"A"`
		Identifier int           `json:"I"`
	}{
		Hub:        hub,
		Method:     method,
		Arguments:  params,
		Identifier: self.nextId,
	}

	self.nextId++

	data, err := json.Marshal(request)
	if err != nil {
		return err
	} else if err := self.socket.WriteMessage(websocket.TextMessage, data); err != nil {
		return err
	}

	return nil
}

func NewWebsocketClient(scheme, host string, hubs []string) (*Client, error) {
	// Negotiate parameters.
	params, err := negotiate(scheme, host)
	if err != nil {
		return nil, err
	}

	// Connect Websocket.
	ws, err := connectWebsocket(host, params, hubs)
	if err != nil {
		return nil, err
	}

	return &Client{
		params: params,
		socket: ws,
		nextId: 1,
	}, nil
}
