package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"example.com/m/wowza"
	"github.com/pion/webrtc/v3"
)

type sessionChans struct {
	offer        chan webrtc.SessionDescription
	answer       chan webrtc.SessionDescription
	iceCandidate chan webrtc.ICECandidateInit
}

const (
	audioFileName = "output.ogg"
	videoFileName = "output.ivf"
)

var (
	videoTrack *webrtc.TrackLocalStaticSample
	audioTrack *webrtc.TrackLocalStaticSample

	sessions = map[string]sessionChans{}
)

func main() {
	videoTrack, audioTrack = startAudioAndVideo(videoFileName, audioFileName)
	startServer(sessions)
}

func startServer(chanMap map[string]sessionChans) {
	onGetOffer := func(req wowza.WsRequest) (wowza.SdpContainer, string, error) {
		fmt.Println("ws: got offer request")

		sessionID := strconv.Itoa(int(time.Now().Unix()))
		session := sessionChans{
			iceCandidate: make(chan webrtc.ICECandidateInit, 50),
			offer:        make(chan webrtc.SessionDescription),
			answer:       make(chan webrtc.SessionDescription),
		}
		chanMap[sessionID] = session

		fmt.Println("ws: start session", sessionID)

		go startConnection(session)

		offer := <-session.offer
		return wowza.ConvertSessionDescription(offer), sessionID, nil
	}
	onSendResponse := func(req wowza.WsRequest) ([]webrtc.ICECandidateInit, error) {
		fmt.Println("ws: got response")

		sessionID := req.StreamInfo.SessionID
		session, ok := chanMap[sessionID]
		if !ok {
			return nil, fmt.Errorf("no matching session id: %v", sessionID)
		}

		session.answer <- wowza.ConvertSdpContainer(req.Sdp)

		iceCandidates := []webrtc.ICECandidateInit{}
	Loop:
		for {
			select {
			case ic := <-session.iceCandidate:
				iceCandidates = append(iceCandidates, ic)
			default:
				break Loop
			}
		}
		return iceCandidates, nil
	}
	wsServerAddr := "localhost:8080"
	fmt.Println("ws server started on", wsServerAddr)
	startWsServer(wsServerAddr, onGetOffer, onSendResponse)
}

func startAudioAndVideo(videoFileName, audioFileName string) (*webrtc.TrackLocalStaticSample, *webrtc.TrackLocalStaticSample) {
	_, err := os.Stat(videoFileName)
	haveVideoFile := !os.IsNotExist(err)

	_, err = os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile && !haveVideoFile {
		panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}

	var (
		videoTrack *webrtc.TrackLocalStaticSample
		audioTrack *webrtc.TrackLocalStaticSample
	)

	var wg sync.WaitGroup
	wg.Add(2)

	if haveVideoFile {
		// Create a video track
		track, err := createVideoTrack()
		if err != nil {
			panic(err)
		}
		go streamVideo(videoFileName, track, &wg)
		videoTrack = track
	}

	if haveAudioFile {
		// Create a audio track
		track, err := createAudioTrack()
		if err != nil {
			panic(err)
		}
		go streamAudio(audioFileName, track, &wg)
		audioTrack = track
	}

	return videoTrack, audioTrack
}

func startConnection(session sessionChans) {
	fmt.Println("start connection")

	finishChan := make(chan struct{})

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	})
	if err != nil {
		panic(err)
	}

	if videoTrack != nil {
		_, err := peerConnection.AddTrack(videoTrack)
		if err != nil {
			panic(err)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		// go func() {
		// 	rtcpBuf := make([]byte, 1500)
		// 	for {
		// 		if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
		// 			return
		// 		}
		// 	}
		// }()
	}

	if audioTrack != nil {
		_, err := peerConnection.AddTrack(audioTrack)
		if err != nil {
			panic(err)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		// go func() {
		// 	rtcpBuf := make([]byte, 1500)
		// 	for {
		// 		if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
		// 			return
		// 		}
		// 	}
		// }()
	}

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			fmt.Println("OnICECandidate", i)
			session.iceCandidate <- i.ToJSON()
		}
	})

	// iceConnectedCtx, iceConnectedCtxCancel := context.WithCancel(context.Background())

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			// iceConnectedCtxCancel()
		}
	})

	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		switch pcs.String() {
		case "failed", "closed":
			fmt.Println("closing WebRTC connection")
			finishChan <- struct{}{}
		}
	})

	// Wait for the offer to be pasted
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Set the remote SessionDescription
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}
	fmt.Println("SetLocalDescription")

	// Create answer
	session.offer <- offer
	fmt.Println("sending offer")

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete
	fmt.Println("gatherComplete")

	// Output the answer in base64 so we can paste it in browser
	fmt.Println("waiting for answer")
	answer := <-session.answer
	fmt.Println("received answer")

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}
	fmt.Println("SetRemoteDescription")

	// block
	<-finishChan
}
