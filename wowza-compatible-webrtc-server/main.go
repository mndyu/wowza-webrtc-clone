package main

import (
	"fmt"
	"os"
	"sync"

	"example.com/m/wowza"
	"github.com/pion/webrtc/v3"
)

const (
	audioFileName = "output.ogg"
	videoFileName = "output.ivf"
)

func main() {
	iceCandidateC := make(chan webrtc.ICECandidateInit, 10)
	offerC := make(chan webrtc.SessionDescription)
	answerC := make(chan webrtc.SessionDescription)

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i != nil {
			fmt.Println("OnICECandidate", i)
			iceCandidateC <- i.ToJSON()
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

	startServer(offerC, answerC, iceCandidateC)
	startAudioAndVideo(peerConnection, videoFileName, audioFileName)

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
	offerC <- offer
	fmt.Println("sending offer")

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete
	fmt.Println("gatherComplete")

	// Output the answer in base64 so we can paste it in browser
	answer := <-answerC
	fmt.Println("received answer")

	// Sets the LocalDescription, and starts our UDP listeners
	if err = peerConnection.SetRemoteDescription(answer); err != nil {
		panic(err)
	}
	fmt.Println("SetRemoteDescription")

	// Block forever
	select {}
}

func startServer(offerC, answerC chan webrtc.SessionDescription, iceCandidateC chan webrtc.ICECandidateInit) {
	onGetOffer := func(req wowza.WsRequest) wowza.SdpContainer {
		offer := <-offerC
		return wowza.ConvertSessionDescription(offer)
	}
	onSendResponse := func(req wowza.WsRequest) []webrtc.ICECandidateInit {
		answerC <- wowza.ConvertSdpContainer(req.Sdp)

		iceCandidates := []webrtc.ICECandidateInit{}
	Loop:
		for {
			select {
			case ic := <-iceCandidateC:
				iceCandidates = append(iceCandidates, ic)
			default:
				break Loop
			}
		}
		return iceCandidates
	}
	wsServerAddr := "localhost:8080"
	go startWsServer(wsServerAddr, onGetOffer, onSendResponse)
	fmt.Println("ws server started on", wsServerAddr)
}

func startAudioAndVideo(pc *webrtc.PeerConnection, videoFile, audioFile string) {
	_, err := os.Stat(videoFileName)
	haveVideoFile := !os.IsNotExist(err)

	_, err = os.Stat(audioFileName)
	haveAudioFile := !os.IsNotExist(err)

	if !haveAudioFile && !haveVideoFile {
		panic("Could not find `" + audioFileName + "` or `" + videoFileName + "`")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	if haveVideoFile {
		// Create a video track
		videoTrack, videoTrackErr := createVideoTrack()
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		rtpSender, videoTrackErr := pc.AddTrack(videoTrack)
		if videoTrackErr != nil {
			panic(videoTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		go streamVideo(videoFileName, videoTrack, &wg)
	}

	if haveAudioFile {
		// Create a audio track
		audioTrack, audioTrackErr := createAudioTrack()
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		rtpSender, audioTrackErr := pc.AddTrack(audioTrack)
		if audioTrackErr != nil {
			panic(audioTrackErr)
		}

		// Read incoming RTCP packets
		// Before these packets are returned they are processed by interceptors. For things
		// like NACK this needs to be called.
		go func() {
			rtcpBuf := make([]byte, 1500)
			for {
				if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
					return
				}
			}
		}()

		go streamAudio(audioFileName, audioTrack, &wg)
	}
}
