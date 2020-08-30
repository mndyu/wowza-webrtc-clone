package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
)

const (
	rtcpPLIInterval = time.Second * 3
	publicDir       = "./website"
	webServerPort   = "8080"
)

var (
	rooms = []Room{}
)

type Room struct {
	ID                int  `json:"id"`
	Status            bool `json:"status"`
	sdpCallOfferChan  chan string
	sdpCallAnswerChan chan string
	sdpRecvOfferChan  chan string
	sdpRecvAnswerChan chan string
}

func handleCall(w http.ResponseWriter, r *http.Request) {
	log.Println("handleCall")

	room := createRoom()

	log.Println("reading body")

	body, _ := ioutil.ReadAll(r.Body)

	log.Println("starting call")
	go func() {
		err := startCall(room.ID)
		if err != nil {
			log.Printf("error: %s\n", err.Error())
		}
	}()

	fmt.Println(room.ID, room)

	room.sdpCallOfferChan <- string(body)
	answer := <-room.sdpCallAnswerChan

	w.Write([]byte(answer))
}

func handleRecv(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	roomID, err := strconv.Atoi(vars["roomID"])
	if err != nil {
		http.Error(w, "invalid room id", http.StatusBadRequest)
		return
	}
	room, err := getRoom(roomID)
	if err != nil {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	body, _ := ioutil.ReadAll(r.Body)

	room.sdpRecvOfferChan <- string(body)
	answer := <-room.sdpRecvAnswerChan

	w.Write([]byte(answer))
}

func handleRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

func startServer() {

	r := mux.NewRouter()
	r.HandleFunc("/room/call", handleCall)
	r.HandleFunc("/room/{roomID}/recv", handleRecv)
	r.HandleFunc("/rooms", handleRooms)

	fs := http.FileServer(http.Dir(publicDir))
	r.PathPrefix("/").Handler(fs)

	http.Handle("/", r)

	log.Printf("Listening on :%s...\n", webServerPort)
	listRoutes(r)

	err := http.ListenAndServe(":"+webServerPort, nil)
	if err != nil {
		log.Fatal(err)
	}

}

func listRoutes(r *mux.Router) {
	log.Println("routes:")

	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		t, err := route.GetPathTemplate()
		if err != nil {
			return err
		}
		fmt.Println("  -", t)
		return nil
	})
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func encodeBase64JSON(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	// if compress {
	// 	b = zip(b)
	// }

	return base64.StdEncoding.EncodeToString(b)
}
func decodeBase64JSON(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	// if compress {
	// 	b = unzip(b)
	// }

	err = json.Unmarshal(b, obj)
	if err != nil {
		panic(err)
	}
}

func createRoom() Room {
	var roomID int
	if len(rooms) > 0 {
		roomID = rooms[len(rooms)-1].ID + 1
	} else {
		roomID = 1
	}

	newRoom := Room{
		ID:                roomID,
		sdpCallOfferChan:  make(chan string),
		sdpCallAnswerChan: make(chan string),
		sdpRecvOfferChan:  make(chan string),
		sdpRecvAnswerChan: make(chan string),
	}
	rooms = append(rooms, newRoom)
	return newRoom
}

func getRoom(roomID int) (Room, error) {
	for _, r := range rooms {
		if r.ID == roomID {
			return r, nil
		}
	}
	return Room{}, nil
}

func main() {
	startServer()
}

func startCall(roomID int) error {
	log.Printf("calling room %d", roomID)

	room, err := getRoom(roomID)
	if err != nil {
		return fmt.Errorf("room %d not found", roomID)
	}

	log.Println("aa11")

	// Everything below is the Pion WebRTC API, thanks for using it ❤️.
	offer := webrtc.SessionDescription{}
	decodeBase64JSON(<-room.sdpCallOfferChan, &offer)
	// fmt.Println(offer)

	log.Println("bb22")

	// Since we are answering use PayloadTypes declared by offerer
	mediaEngine := webrtc.MediaEngine{}
	err = mediaEngine.PopulateFromSDP(offer)
	if err != nil {
		panic(err)
	}

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))

	peerConnectionConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
	if err != nil {
		panic(err)
	}

	// Allow us to receive 1 video track
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	localTrackChan := make(chan *webrtc.Track)
	// Set a handler for when a new remote track starts, this just distributes all our packets
	// to connected peers
	peerConnection.OnTrack(func(remoteTrack *webrtc.Track, receiver *webrtc.RTPReceiver) {
		fmt.Println("received a track")

		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		// This can be less wasteful by processing incoming RTCP events, then we would emit a NACK/PLI when a viewer requests it
		go func() {
			ticker := time.NewTicker(rtcpPLIInterval)
			for range ticker.C {
				if rtcpSendErr := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: remoteTrack.SSRC()}}); rtcpSendErr != nil {
					fmt.Println(rtcpSendErr)
				}
			}
		}()

		// Create a local track, all our SFU clients will be fed via this track
		localTrack, newTrackErr := peerConnection.NewTrack(remoteTrack.PayloadType(), remoteTrack.SSRC(), "video", "pion")
		if newTrackErr != nil {
			panic(newTrackErr)
		}
		localTrackChan <- localTrack

		rtpBuf := make([]byte, 1400)
		for {
			i, readErr := remoteTrack.Read(rtpBuf)
			if readErr != nil {
				panic(readErr)
			}

			// ErrClosedPipe means we don't have any subscribers, this is ok if no peers have connected yet
			if _, err = localTrack.Write(rtpBuf[:i]); err != nil && err != io.ErrClosedPipe {
				panic(err)
			}
		}
	})

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	log.Println("gathering...")

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Get the LocalDescription and take it to base64 so we can paste in browser
	fmt.Println("received call SDP")
	// fmt.Println(encodeBase64JSON(*peerConnection.LocalDescription()))
	room.sdpCallAnswerChan <- encodeBase64JSON(*peerConnection.LocalDescription())

	localTrack := <-localTrackChan
	for {
		fmt.Println("")
		fmt.Println("Curl an base64 SDP to start sendonly peer connection")

		recvOnlyOffer := webrtc.SessionDescription{}
		decodeBase64JSON(<-room.sdpRecvOfferChan, &recvOnlyOffer)

		// Create a new PeerConnection
		peerConnection, err := api.NewPeerConnection(peerConnectionConfig)
		if err != nil {
			panic(err)
		}

		_, err = peerConnection.AddTrack(localTrack)
		if err != nil {
			panic(err)
		}

		// Set the remote SessionDescription
		err = peerConnection.SetRemoteDescription(recvOnlyOffer)
		if err != nil {
			panic(err)
		}

		// Create answer
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		// Create channel that is blocked until ICE Gathering is complete
		gatherComplete = webrtc.GatheringCompletePromise(peerConnection)

		// Sets the LocalDescription, and starts our UDP listeners
		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}

		// Block until ICE Gathering is complete, disabling trickle ICE
		// we do this because we only can exchange one signaling message
		// in a production application you should exchange ICE Candidates via OnICECandidate
		<-gatherComplete

		// Get the LocalDescription and take it to base64 so we can paste in browser
		fmt.Println("received recv SDP")
		// fmt.Println(encodeBase64JSON(*peerConnection.LocalDescription()))
		room.sdpRecvAnswerChan <- encodeBase64JSON(*peerConnection.LocalDescription())
	}
}
