package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

	signalTunnel *transport.Tunnel // tunnel with the signal server

	peerDiscoveryTunnel *transport.Tunnel // tunnel that serves discovery thread
	answerTunnel        *transport.Tunnel // tunnel that serves answer thread
	offerTunnel         *transport.Tunnel // tunnel that serves offer thread

	IOTunnel *transport.Tunnel // Input/Output tunnel from user

	peers    []string   // A set of peers
	peerLock sync.Mutex // Sync peer read and write

	/*
		peers       map[string]*transport.Tunnel // tunnel for the peer connection
		connMux     sync.Mutex                   // Sync connection handler
		clientClose chan struct{}                // Graceful shutdown for client
	*/
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
	client.signalTunnel = transport.GetTunnel()

	// connect using websocket
	err = client.connectUpstream()
	if err != nil {
		return err
	}

	fmt.Printf("Join new channel with:\n\t -join=%s\n", client.Channel)

	client.answerTunnel = transport.GetTunnel()
	client.peerDiscoveryTunnel = transport.GetTunnel()
	client.offerTunnel = transport.GetTunnel()

	// Start answer routine
	go func() {

		for {
			err = client.answer()
			if err != nil {
				log.Printf(err.Error())
			}
		}
	}()

	// Start peer discovery routine
	go func() {

		for range time.NewTicker(5 * time.Second).C {
			client.peerLock.Lock()
			peers, err := getPeersViaTunnel(client.peerDiscoveryTunnel, client.ID)
			if err != nil {
				log.Printf("failed to get peers, error %v", err)
				client.peerLock.Unlock()
				continue
			}
			client.peers = peers
			client.peerLock.Unlock()
		}
	}()

	// Start offer routine
	go func() {

		for range time.NewTicker(5 * time.Second).C {
			client.peerLock.Lock()
			for _, peer := range client.peers {
				err = client.offer(peer)
				if err != nil {
					log.Printf("failed to connect peer %s, error %v",
						peer, err)
				}
			}
			client.peerLock.Unlock()
		}
	}()

	// Start Multiplexer
	go func() {
		for {
			select {
			case data := <-client.answerTunnel.Outgoing:
				client.signalTunnel.Outgoing <- data

			case data := <-client.peerDiscoveryTunnel.Outgoing:
				client.signalTunnel.Outgoing <- data

			case data := <-client.offerTunnel.Outgoing:
				client.signalTunnel.Outgoing <- data
			}
		}
	}()

	// Start DeMultiplexer
	go func() {
		for {
			data := <-client.signalTunnel.Incoming
			msg, err := transport.UnmarshalMessage(data)
			if err != nil {
				log.Printf("failed to decode message, error %v", err)
				continue
			}
			switch msg.TYP {
			case transport.SDP_OFFER:
				client.answerTunnel.Incoming <- data

			case transport.SDP_ANSWER:
				client.offerTunnel.Incoming <- data

			case transport.SDP_DISCOVER_ANSWER:
				client.peerDiscoveryTunnel.Incoming <- data

			default:
				log.Printf("Invalid message type %s", msg.TYP)
			}
		}
	}()

	return nil
}

// JoinChannel join a existing channel via server
func (client *Client) JoinChannel() error {
	// Create the transport
	client.signalTunnel = transport.GetTunnel()

	// connect using websocket
	err := client.connectUpstream()
	if err != nil {
		return err
	}

	client.answerTunnel = transport.GetTunnel()
	client.peerDiscoveryTunnel = transport.GetTunnel()
	client.offerTunnel = transport.GetTunnel()

	// Start answer routine
	go func() {

		for {
			err = client.answer()
			if err != nil {
				log.Printf(err.Error())
			}
		}
	}()

	// Start peer discovery routine
	go func() {

		for range time.NewTicker(5 * time.Second).C {
			//	client.peerLock.Lock()
			peers, err := getPeersViaTunnel(client.peerDiscoveryTunnel, client.ID)
			if err != nil {
				log.Printf("failed to get peers, error %v", err)
				client.peerLock.Unlock()
				continue
			}
			client.peers = peers
			//	client.peerLock.Unlock()
		}
	}()

	// Start offer routine
	go func() {

		for range time.NewTicker(5 * time.Second).C {
			for _, peer := range client.peers {
				err = client.offer(peer)
				if err != nil {
					log.Printf("failed to connect peer %s, error %v",
						peer, err)
				}
			}
		}
	}()

	// Start Multiplexer
	go func() {
		for {
			select {
			case data := <-client.answerTunnel.Outgoing:
				log.Printf("Data from answer tunnel send outward")
				client.signalTunnel.Outgoing <- data

			case data := <-client.peerDiscoveryTunnel.Outgoing:
				log.Printf("Data from peer discovery tunnel send outward")
				client.signalTunnel.Outgoing <- data

			case data := <-client.offerTunnel.Outgoing:
				log.Printf("Data from offer tunnel send outward")
				client.signalTunnel.Outgoing <- data
			}
		}
	}()

	// Start DeMultiplexer
	go func() {
		for {
			data := <-client.signalTunnel.Incoming
			msg, err := transport.UnmarshalMessage(data)
			if err != nil {
				log.Printf("failed to decode message, error %v", err)
				continue
			}
			switch msg.TYP {
			case transport.SDP_OFFER:
				client.answerTunnel.Incoming <- data

			case transport.SDP_ANSWER:
				client.offerTunnel.Incoming <- data

			case transport.SDP_DISCOVER_ANSWER:
				client.peerDiscoveryTunnel.Incoming <- data

			default:
				log.Printf("Invalid message type %s", msg.TYP)
			}
		}
	}()

	return nil
}

// offer a webrtc request to a peer via Server
func (client *Client) offer(peer string) error {
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

		for {
			data := <-client.IOTunnel.Outgoing
			message := string(data)
			// Send the message as text
			err := dataChannel.SendText(message[:len(message)-1])
			if err != nil {
				panic(err)
			}
		}
	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		client.IOTunnel.Incoming <- msg.Data
		//fmt.Printf("Message from '%s': '%s'\n", dataChannel.Label, string(msg.Data))
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
	answer, err := exchangeOfferViaTunnel(client.ID, peer, offer, client.signalTunnel)
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

			for {
				data := <-client.IOTunnel.Outgoing
				message := string(data)
				// Send the message as text
				err := dataChannel.SendText(message[:len(message)-1])
				if err != nil {
					panic(err)
				}
			}
		})

		// Register text message handling
		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			client.IOTunnel.Incoming <- msg.Data
			//fmt.Printf("Message from '%s': '%s'\n", dataChannel.Label, string(msg.Data))
		})
	})

	// Exchange the offer/answer via HTTP
	offerChan, answerChan, err := exchangeAnswerViaTunnel(client.answerTunnel)
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
					c.signalTunnel.Incoming <- message // Send message to the incoming channel
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
			case message := <-c.signalTunnel.Outgoing:
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
		c.signalTunnel.Close()
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

// getPeersViaTunnel query about peer client on the same channel via signalTunnel
func getPeersViaTunnel(peerTunnel *transport.Tunnel, cid string) (peers []string, err error) {
	data, err := transport.CreateDiscoverQuery(cid).Marshal()
	if err != nil {
		return
	}

	// Send the query
	peerTunnel.Outgoing <- data

	// Wait for the answer
	data = <-peerTunnel.Incoming
	message, err := transport.UnmarshalMessage(data)
	if err != nil {
		return
	}
	peers = message.ANS
	return
}

// exchangeOfferViaTunnel exchange the SDP offer and answer via signalTunnel.
func exchangeOfferViaTunnel(src string, dest string, offer webrtc.SessionDescription, offerTunnel *transport.Tunnel) (answer webrtc.SessionDescription, err error) {

	data, err := transport.CreateOffer(src, dest, &offer).Marshal()
	if err != nil {
		return
	}

	log.Printf("Sending offer for %s", dest)

	// Send the offer
	offerTunnel.Outgoing <- data

	log.Printf("Waiting for answer from %s", dest)

	// Wait for the answer
	data = <-offerTunnel.Incoming
	message, err := transport.UnmarshalMessage(data)
	if err != nil {
		return
	}
	fmt.Println(message)
	answer = *message.SDP
	return
}

// exchangeAnswerViaTunnel exchange the SDP offer and answer via signalTunnel
func exchangeAnswerViaTunnel(answerTunnel *transport.Tunnel) (
	offerOut chan webrtc.SessionDescription, answerIn chan webrtc.SessionDescription, err error) {

	offerOut = make(chan webrtc.SessionDescription)
	answerIn = make(chan webrtc.SessionDescription)

	go func() {
		data := <-answerTunnel.Incoming

		message, err := transport.UnmarshalMessage(data)
		if err != nil {
			return
		}

		log.Printf("Got offer from %s", message.SRC)

		offerOut <- *message.SDP
		answer := <-answerIn

		data, err = transport.CreateAnswer(message.DST, message.SRC, &answer).Marshal()
		if err != nil {
			return
		}

		log.Printf("Got offer sending answer to %s", message.SRC)

		answerTunnel.Outgoing <- data
	}()

	return
}
