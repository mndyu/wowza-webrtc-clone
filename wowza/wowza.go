package wowza

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type StreamInfo struct {
	ApplicationName string `json:"applicationName"`
	StreamName      string `json:"streamName"`
	SessionID       string `json:"sessionId"`
}

type SdpContainer struct {
	Type string `json:"type"`
	Sdp  string `json:"sdp"`
}

type WsRequest struct {
	Direction  string       `json:"direction"`
	Command    string       `json:"command"`
	StreamInfo StreamInfo   `json:"streamInfo"`
	Sdp        SdpContainer `json:"sdp,omitempty"`
}

type WsResponse struct {
	Status            int                       `json:"status"`
	StatusDescription string                    `json:"statusDescription"`
	Direction         string                    `json:"direction"`
	Command           string                    `json:"command"`
	StreamInfo        StreamInfo                `json:"streamInfo"`
	Sdp               SdpContainer              `json:"sdp,omitempty"`
	ICECandidates     []webrtc.ICECandidateInit `json:"iceCandidates,omitempty"`
}

func ConvertSdpContainer(s SdpContainer) webrtc.SessionDescription {
	return webrtc.SessionDescription{
		Type: webrtc.NewSDPType(s.Type),
		SDP:  s.Sdp,
	}
}

func ConvertSessionDescription(s webrtc.SessionDescription) SdpContainer {
	return SdpContainer{
		Type: s.Type.String(),
		Sdp:  s.SDP,
	}
}

func SendWsRequest(req WsRequest) (WsResponse, error) {
	u := url.URL{Scheme: "wss", Host: "vrlive.newjivr.com", Path: "/webrtc-session.json"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return WsResponse{}, err
	}
	defer c.Close()

	reqJson, err := json.Marshal(req)
	if err != nil {
		return WsResponse{}, fmt.Errorf("request json format error: %w", err)
	}

	err = c.WriteMessage(websocket.TextMessage, reqJson)
	if err != nil {
		return WsResponse{}, fmt.Errorf("write: %w", err)
	}
	fmt.Println(string(reqJson))

	_, message, err := c.ReadMessage()
	if err != nil {
		return WsResponse{}, fmt.Errorf("read: %w", err)
	}

	var resp WsResponse
	json.Unmarshal(message, &resp)

	return resp, nil
}

func GetOfferSdp(app, stream string) (string, string, error) {
	req := WsRequest{
		Direction: "play",
		Command:   "getOffer",
		StreamInfo: StreamInfo{
			ApplicationName: app,
			StreamName:      stream,
		},
	}

	resp, err := SendWsRequest(req)
	if err != nil {
		return "", "", err
	}

	if resp.Status != 200 {
		return "", "", fmt.Errorf("invalid response status: %v", resp)
	}

	sdpType := resp.Sdp.Type
	sdp := resp.Sdp.Sdp

	fmt.Println(resp)

	if sdpType == "offer" && sdp != "" {
		return sdp, resp.StreamInfo.SessionID, nil
	}

	return "", "", fmt.Errorf("invalid response: %v", resp)
}

func SendAnswerSdp(app, stream, sessionId string, sdp webrtc.SessionDescription) ([]webrtc.ICECandidateInit, error) {
	sdpObject := SdpContainer{
		"answer",
		sdp.SDP,
	}
	req := WsRequest{
		Direction: "play",
		Command:   "sendResponse",
		StreamInfo: StreamInfo{
			ApplicationName: app,
			StreamName:      stream,
			SessionID:       sessionId,
		},
		Sdp: sdpObject,
	}

	resp, err := SendWsRequest(req)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		return nil, fmt.Errorf("invalid response status: %v", resp)
	}

	return resp.ICECandidates, nil
}
