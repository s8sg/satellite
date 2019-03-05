package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pions/webrtc"
)

type Server struct {
	Port int // Port to host the HTTP Server
}

func (server *Server) Serve() error {

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return err
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label, d.ID)

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label, d.ID)

			for range time.NewTicker(5 * time.Second).C {
				t := time.Now()
				message := t.String()
				fmt.Printf("Sending '%s'\n", message)

				// Send the message as text
				err := d.SendText(message)
				if err != nil {
					// TODO handle timeout
					panic(err)
				}
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from DataChannel '%s': '%s'\n", d.Label, string(msg.Data))
		})
	})

	// Exchange the offer/answer via HTTP
	addr := fmt.Sprintf(":%d", server.Port)
	offerChan, answerChan, err := exchangeOfferAndAnswer(addr)
	if err != nil {
		return err
	}

	// Wait for the remote SessionDescription
	offer := <-offerChan

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		return err
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		return err
	}

	// Send the answer
	answerChan <- answer

	// Block forever
	select {}
}

// exchangeOfferAndAnswer exchange the SDP offer and answer using an HTTP server.
func exchangeOfferAndAnswer(address string) (
	offerOut chan webrtc.SessionDescription, answerIn chan webrtc.SessionDescription, err error) {

	offerOut = make(chan webrtc.SessionDescription)
	answerIn = make(chan webrtc.SessionDescription)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var offer webrtc.SessionDescription
		err = json.NewDecoder(r.Body).Decode(&offer)
		if err != nil {
			return
		}

		offerOut <- offer
		answer := <-answerIn

		err = json.NewEncoder(w).Encode(answer)
		if err != nil {
			return
		}

	})

	go http.ListenAndServe(address, nil)
	fmt.Println("Listening on", address)

	return
}
