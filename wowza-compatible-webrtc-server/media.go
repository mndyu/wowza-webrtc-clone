package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/ivfreader"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
)

func createVideoTrack() (*webrtc.TrackLocalStaticSample, error) {
	return webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion")
}

func streamVideo(filename string, track *webrtc.TrackLocalStaticSample, wg *sync.WaitGroup) error {
	fmt.Println("video open")

	// Open a IVF file and start reading using our IVFReader
	file, ivfErr := os.Open(filename)
	if ivfErr != nil {
		panic(ivfErr)
	}

	wg.Done()
	wg.Wait()
	time.Sleep(500 * time.Millisecond)

Loop:
	for {
		fmt.Println("video start")
		wg.Add(1)

		ivf, header, ivfErr := ivfreader.NewWith(file)
		if ivfErr != nil {
			panic(ivfErr)
		}

		// Wait for connection established
		// <-iceConnectedCtx.Done()

		// Send our video file frame at a time. Pace our sending so we send it at the same speed it should be played back as.
		// This isn't required since the video is timestamped, but we will such much higher loss if we send all at once.
		sleepTime := time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000)
		for {
			frame, _, ivfErr := ivf.ParseNextFrame()
			if ivfErr == io.EOF {
				file.Seek(0, io.SeekStart)
				fmt.Println("video end")
				wg.Done()
				wg.Wait()
				time.Sleep(500 * time.Millisecond)
				continue Loop
			}

			if ivfErr != nil {
				panic(ivfErr)
			}

			time.Sleep(sleepTime)
			if ivfErr = track.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
				panic(ivfErr)
			}
		}
	}
}

func createAudioTrack() (*webrtc.TrackLocalStaticSample, error) {
	return webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion")
}

func streamAudio(filename string, track *webrtc.TrackLocalStaticSample, wg *sync.WaitGroup) error {
	fmt.Println("audio open")

	// Open a IVF file and start reading using our IVFReader
	file, oggErr := os.Open(filename)
	if oggErr != nil {
		panic(oggErr)
	}

	wg.Done()
	wg.Wait()
	time.Sleep(500 * time.Millisecond)

Loop:
	for {
		fmt.Println("audio start")
		wg.Add(1)

		// Open on oggfile in non-checksum mode.
		ogg, _, oggErr := oggreader.NewWith(file)
		if oggErr != nil {
			panic(oggErr)
		}

		// Wait for connection established
		// <-iceConnectedCtx.Done()

		// Keep track of last granule, the difference is the amount of samples in the buffer
		var lastGranule uint64
		for {
			pageData, pageHeader, oggErr := ogg.ParseNextPage()
			if oggErr == io.EOF {
				// fmt.Printf("All audio pages parsed and sent")
				file.Seek(0, io.SeekStart)
				lastGranule = 0
				fmt.Println("audio end")
				wg.Done()
				wg.Wait()
				time.Sleep(500 * time.Millisecond)
				continue Loop
			}

			if oggErr != nil {
				panic(oggErr)
			}

			// The amount of samples is the difference between the last and current timestamp
			sampleCount := float64(pageHeader.GranulePosition - lastGranule)
			lastGranule = pageHeader.GranulePosition
			sampleDuration := time.Duration((sampleCount/48000)*1000) * time.Millisecond

			if oggErr = track.WriteSample(media.Sample{Data: pageData, Duration: sampleDuration}); oggErr != nil {
				panic(oggErr)
			}

			time.Sleep(sampleDuration)
		}
	}
}
