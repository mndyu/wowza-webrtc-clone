package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"

	"example.com/m/wowza"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func startWsServer(addr string, onGetOffer func(wowza.WsRequest) wowza.SdpContainer, onSendResponse func(wowza.WsRequest) []webrtc.ICECandidateInit) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Print("upgrade:", err)
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				break
			}
			// log.Printf("recv: %s", message)

			var req wowza.WsRequest
			err = json.Unmarshal(message, &req)
			if err != nil {
				fmt.Printf("error request json unmarshal: %v\n", err)
			}

			var res wowza.WsResponse

			switch req.Command {
			case "getOffer":
				offerSdp := onGetOffer(req)
				res = wowza.WsResponse{
					Status:            200,
					StatusDescription: "OK",
					Direction:         "play",
					Command:           "getOffer",
					StreamInfo:        req.StreamInfo,
					Sdp:               offerSdp,
				}
			case "sendResponse":
				iceCandidates := onSendResponse(req)
				res = wowza.WsResponse{
					Status:            200,
					StatusDescription: "OK",
					Direction:         "play",
					Command:           "sendResponse",
					StreamInfo:        req.StreamInfo,
					ICECandidates:     iceCandidates,
				}
			}

			resMessage, err := json.Marshal(res)
			if err != nil {
				fmt.Printf("error response json marshal: %v\n", err)
			}

			err = c.WriteMessage(mt, resMessage)
			if err != nil {
				log.Println("write:", err)
				break
			}
		}
	}

	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/", handler)
	http.HandleFunc("/webrtc-session.json", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
