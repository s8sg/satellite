package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pions/webrtc"
)

type Client struct {
	Remote string // Remote server that has the HRRP Server
}

func (client *Client) Connect() error {
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

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		return err
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label, dataChannel.ID)

		for range time.NewTicker(5 * time.Second).C {
			t := time.Now()
			message := t.String()
			fmt.Printf("Sending '%s'\n", message)

			// Send the message as text
			err := dataChannel.SendText(message)
			if err != nil {
				// TODO: Handle timeout
				panic(err)
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label, string(msg.Data))
	})

	// Create an offer to send to the browser
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		return err
	}

	fmt.Println(offer)

	// Exchange the offer for the answer
	answer, err := exchangeOfferAndAnswer(offer, client.Remote)
	if err != nil {
		return err
	}

	fmt.Println(answer)

	// Apply the answer as the remote description
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		return err
	}

	// Block forever
	select {}
}

// exchangeOfferAndAnswer exchange the SDP offer and answer using an HTTP Post request.
func exchangeOfferAndAnswer(offer webrtc.SessionDescription, address string) (answer webrtc.SessionDescription, err error) {

	b := new(bytes.Buffer)
	err = json.NewEncoder(b).Encode(offer)
	if err != nil {
		return
	}

	resp, err := http.Post("http://"+address, "application/json; charset=utf-8", b)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&answer)
	if err != nil {
		return
	}

	return
}
