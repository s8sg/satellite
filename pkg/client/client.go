package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pions/webrtc"
	"github.com/pkg/errors"

	"github.com/s8sg/satellite/pkg/transport"
)

var httpClient *http.Client

type Client struct {
	Remote string // Remote server that has the HRRP Server
	ID     string // Channel Id to join
	tunnel *transport.Tunnel
}

// CreateChannel creates a channel from server and join
func (client *Client) CreateChannel() (err error) {

	// create a channel and get channel id from server
	resp, err := http.Get("http://" + client.Remote + "/create")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// read the channel ID
	channelId, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	client.ID = string(channelId)

	// Create the transport
	client.tunnel = transport.GetTunnel()

	// connect using websocket
	err = client.ConnectUpstream()
	if err != nil {
		return err
	}

	fmt.Printf("Join new channel with:\n\t -join=%s\n", client.ID)

	// Wait to answer offer from peer
	err = client.answer()
	if err != nil {
		return err
	}

	select {}

}

// JoinChannel join a existing channel via server
func (client *Client) JoinChannel() error {
	// Create the transport
	client.tunnel = transport.GetTunnel()

	// connect using websocket
	err := client.ConnectUpstream()
	if err != nil {
		return err
	}

	// Send offer to peer
	err = client.offer()
	if err != nil {
		return err
	}

	select {}
}

// offer a webrtc request to a peer via Server
func (client *Client) offer() error {
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

	// Exchange the offer for the answer
	answer, err := exchangeOfferViaTunnel(offer, client.tunnel)
	if err != nil {
		return err
	}

	// Apply the answer as the remote description
	err = peerConnection.SetRemoteDescription(answer)
	if err != nil {
		return err
	}

	return nil
}

// answer a webrtc request from peer via Server
func (client *Client) answer() error {
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
	offerChan, answerChan, err := exchangeAnswerViaTunnel(client.tunnel)
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

	return nil
}

// ConnectUpstream connect and serve exchnages from WS
func (c *Client) ConnectUpstream() error {

	httpClient = http.DefaultClient
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	remote := c.Remote
	if !strings.HasPrefix(remote, "ws") {
		remote = "ws://" + remote
	}

	remoteURL, urlErr := url.Parse(remote)
	if urlErr != nil {
		return errors.Wrap(urlErr, "bad remote URL")
	}

	u := url.URL{Scheme: remoteURL.Scheme, Host: remoteURL.Host, Path: "/tunnel"}

	log.Printf("Creating SDP tunnel via %s", u.String())

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{
		"Authorization": []string{"Bearer " + c.ID},
	})
	if err != nil {
		return err
	}

	log.Printf("SDP tunnel established: %s", ws.LocalAddr())

	connectionDone := make(chan struct{})
	go func() {
		for {
			messageType, message, err := ws.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}

			switch messageType {
			case websocket.TextMessage:
				log.Printf("TextMessage: %s\n", message)

			case websocket.BinaryMessage:
				c.tunnel.Incoming <- message // Send message to the incoming channel
			}

		}
	}()

	go func() {
		for {
			message := <-c.tunnel.Outgoing // Get message from the outgoing channel

			err := ws.WriteMessage(websocket.BinaryMessage, message)
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}()

	// close connection to server once exchnage is done
	go func() {
		<-connectionDone
		ws.Close()
		c.tunnel.Close()
	}()

	return nil
}

// exchangeOfferViaTunnel exchange the SDP offer and answer via tunnel.
func exchangeOfferViaTunnel(offer webrtc.SessionDescription, tunnel *transport.Tunnel) (answer webrtc.SessionDescription, err error) {

	message, err := json.Marshal(offer)
	if err != nil {
		return
	}

	// Send the offer
	tunnel.Outgoing <- message

	// Wait for the answer
	message = <-tunnel.Incoming
	err = json.Unmarshal(message, &answer)

	return
}

// exchangeAnswerViaTunnel exchange the SDP offer and answer via tunnel
func exchangeAnswerViaTunnel(tunnel *transport.Tunnel) (
	offerOut chan webrtc.SessionDescription, answerIn chan webrtc.SessionDescription, err error) {

	offerOut = make(chan webrtc.SessionDescription)
	answerIn = make(chan webrtc.SessionDescription)

	go func() {
		message := <-tunnel.Incoming

		var offer webrtc.SessionDescription
		err = json.Unmarshal(message, &offer)
		if err != nil {
			return
		}

		offerOut <- offer
		answer := <-answerIn

		message, err = json.Marshal(answer)
		if err != nil {
			return
		}

		tunnel.Outgoing <- message
	}()

	return
}
