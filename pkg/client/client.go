package client

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pions/webrtc"
	"github.com/pkg/errors"

	"github.com/s8sg/satellite/pkg/transport"
)

var httpClient *http.Client

type Client struct {
	Remote  string // Remote server that has the HRRP Server
	Port    int    // Port from which UI will be served
	ID      string // Client Id
	Channel string // Channel Id to join
	tunnel  *transport.Tunnel
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
	client.Channel = string(channelId)

	// Create the transport
	client.tunnel = transport.GetTunnel()

	// connect using websocket
	err = client.connectUpstream()
	if err != nil {
		return err
	}

	fmt.Printf("Join new channel with:\n\t -join=%s\n", client.Channel)

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
	err := client.connectUpstream()
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
	// Discover peers via tunnel
	peers, err := getPeersViaTunnel(client.tunnel, client.ID)
	if err != nil {
		return err
	}

	for _, peer := range peers {

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
		dataChannel, err := peerConnection.CreateDataChannel(client.Channel, nil)
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
			fmt.Printf("Data channel '%s'-'%d' open. Typed messages will now be sent to any connected DataChannels\n", dataChannel.Label, dataChannel.ID)

			reader := bufio.NewReader(os.Stdin)
			for {
				message, _ := reader.ReadString('\n')
				// Send the message as text
				err := dataChannel.SendText(message[:len(message)-1])
				if err != nil {
					// TODO: Handle timeout
					panic(err)
				}
			}
		})

		// Register text message handling
		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from '%s': '%s'\n", dataChannel.Label, string(msg.Data))
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
		answer, err := exchangeOfferViaTunnel(client.ID, peer, offer, client.tunnel)
		if err != nil {
			return err
		}

		// Apply the answer as the remote description
		err = peerConnection.SetRemoteDescription(answer)
		if err != nil {
			return err
		}
	}

	return nil
}

// answer answer a webrtc request from peer via Server
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
	peerConnection.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", dataChannel.Label, dataChannel.ID)

		// Register channel opening handling
		dataChannel.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Typed messages will now be sent to any connected DataChannels\n", dataChannel.Label, dataChannel.ID)

			reader := bufio.NewReader(os.Stdin)
			for {
				message, _ := reader.ReadString('\n')
				// Send the message as text
				err := dataChannel.SendText(message[:len(message)-1])
				if err != nil {
					// TODO: Handle timeout
					panic(err)
				}
			}
		})

		// Register text message handling
		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Printf("Message from '%s': '%s'\n", dataChannel.Label, string(msg.Data))
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
func (c *Client) connectUpstream() error {

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
		"Authorization": []string{"Bearer " + c.Channel + "-" + c.ID},
	})
	if err != nil {
		return err
	}

	log.Printf("SDP tunnel established: %s", ws.LocalAddr())

	connectionDone := make(chan int)
	go func() {
		defer func() {
			close(connectionDone)
		}()
		for {
			select {

			case <-connectionDone:
				return
			default:
				messageType, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					fmt.Println("Stopping Inbound Handler")
					return
				}

				switch messageType {
				case websocket.TextMessage:
					log.Printf("TextMessage: %s\n", message)

				case websocket.BinaryMessage:
					c.tunnel.Incoming <- message // Send message to the incoming channel
				}
			}

		}
	}()

	stopListening := make(chan int)
	go func() {
		defer func() {
			close(stopListening)
		}()
		for {
			select {
			case <-stopListening:
				return
			case message := <-c.tunnel.Outgoing:
				err := ws.WriteMessage(websocket.BinaryMessage, message)
				if err != nil {
					log.Println("write:", err)
					fmt.Println("Stopping Outbound Handler")
					return
				}
			}
		}
	}()

	// close connection to server once exchnage is done
	go func() {
		select {
		case <-connectionDone:
			fmt.Println("Stopping Outbound Handler")
			stopListening <- 0
		case <-stopListening:
			fmt.Println("Stopping Inbound Handler")
			connectionDone <- 0
		}
		ws.Close()
		c.tunnel.Close()
		for range time.NewTicker(5 * time.Second).C {
			// Initiate a new connection to server
			fmt.Println("Connection Lost with Server, retrying connection..")
			err = c.connectUpstream()
			if err == nil {
				return
			}
		}
	}()

	return nil
}

// getPeersViaTunnel query about peer client on the same channel via tunnel
func getPeersViaTunnel(tunnel *transport.Tunnel, cid string) (peers []string, err error) {
	data, err := transport.CreateDiscoverQuery(cid).Marshal()
	if err != nil {
		return
	}

	// Send the query
	tunnel.Outgoing <- data

	// Wait for the answer
	data = <-tunnel.Incoming
	message, err := transport.UnmarshalMessage(data)
	if err != nil {
		return
	}
	peers = message.ANS
	return
}

// exchangeOfferViaTunnel exchange the SDP offer and answer via tunnel.
func exchangeOfferViaTunnel(src string, dest string, offer webrtc.SessionDescription, tunnel *transport.Tunnel) (answer webrtc.SessionDescription, err error) {

	data, err := transport.CreateOffer(src, dest, &offer).Marshal()
	if err != nil {
		return
	}

	// Send the offer
	tunnel.Outgoing <- data

	// Wait for the answer
	data = <-tunnel.Incoming
	message, err := transport.UnmarshalMessage(data)
	if err != nil {
		return
	}
	answer = *message.SDP
	return
}

// exchangeAnswerViaTunnel exchange the SDP offer and answer via tunnel
func exchangeAnswerViaTunnel(tunnel *transport.Tunnel) (
	offerOut chan webrtc.SessionDescription, answerIn chan webrtc.SessionDescription, err error) {

	offerOut = make(chan webrtc.SessionDescription)
	answerIn = make(chan webrtc.SessionDescription)

	go func() {
		data := <-tunnel.Incoming

		message, err := transport.UnmarshalMessage(data)
		if err != nil {
			return
		}

		offerOut <- *message.SDP
		answer := <-answerIn

		data, err = transport.CreateAnswer(message.DST, message.SRC, &answer).Marshal()
		if err != nil {
			return
		}

		tunnel.Outgoing <- data
	}()

	return
}
