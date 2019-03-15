package transport

import (
	"encoding/json"
	"fmt"
	"github.com/pions/webrtc"
	"log"
	"sync"
)

// Message a wrapper on top of SDP
type Message struct {
	TYP string                     `json:"typ, omitempty"` // Signal Type (OFFER, ANSWER)
	SRC string                     `json:"src, omitempty"` // Source (Name)
	DST string                     `json:"dst, omitempty"` // Dest (Name)
	SDP *webrtc.SessionDescription `json:"msg, omitempty"` // SIGNAL (SDP Msg)
	ANS []string                   `json:"ans, omitempty"` // Discover Answer
}

// Message types
const (
	SDP_OFFER           = "OFFER"        // SDP offer signal
	SDP_ANSWER          = "ANSWER"       // SDP answer signal
	SDP_DISCOVER_QUERY  = "DISCOVER_QRY" // SDP discover query
	SDP_DISCOVER_ANSWER = "DISCOVER_ANS" // SDP discover ans
)

// CreateOffer create a message wrapping a offer signal
func CreateOffer(src string, dest string, offer *webrtc.SessionDescription) *Message {
	msg := &Message{
		TYP: SDP_OFFER,
		SRC: src,
		DST: dest,
		SDP: offer,
		ANS: nil,
	}
	return msg
}

// CreateAnswer create a message wrapping an answer signal
func CreateAnswer(src string, dest string, answer *webrtc.SessionDescription) *Message {
	msg := &Message{
		TYP: SDP_ANSWER,
		SRC: src,
		DST: dest,
		SDP: answer,
		ANS: nil,
	}
	return msg
}

// CreateDiscoverQuery create a message wrapping a discover query
func CreateDiscoverQuery(src string) *Message {
	msg := &Message{
		TYP: SDP_DISCOVER_QUERY,
		SRC: src,
		DST: "",
		SDP: nil,
		ANS: nil,
	}
	return msg
}

// CreateDiscoverAnswer create a message wrapping a discover answer
func CreateDiscoverAnswer(clients []string, dest string) *Message {
	msg := &Message{
		TYP: SDP_DISCOVER_ANSWER,
		SRC: "",
		DST: dest,
		ANS: clients,
		SDP: nil,
	}
	return msg
}

// Marshal marshal a message
func (msg *Message) Marshal() ([]byte, error) {
	data, err := json.Marshal(msg)
	return data, err
}

// UnmarshalMessage unmarshal a message
func UnmarshalMessage(data []byte) (*Message, error) {
	msg := &Message{}
	err := json.Unmarshal(data, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// Tunnel client/server side endpoint for a connection
type Tunnel struct {
	Accquired bool // State if a tunnel is serving any client
	Incoming  chan []byte
	Outgoing  chan []byte

	tunnelDone chan struct{}
}

// GetTunnel return a new instance of a tunnel
func GetTunnel() *Tunnel {
	t := &Tunnel{
		Accquired: false,
		Incoming:  make(chan []byte),
		Outgoing:  make(chan []byte),
	}
	return t
}

// Close close a tunnel
func (t *Tunnel) Close() {
	close(t.Incoming)
	close(t.Outgoing)
}

// Router routes request between tunnels in server
type Router struct {
	ChannelId    string             // Identifiaction of the channel
	tunnels      []*Tunnel          // tunnels within a router
	mapping      map[string]*Tunnel // maps a client to a tunnel
	tunnelsMux   sync.Mutex         // Sync lock
	activeTunnel int                // no of active tunnel
	RouterDone   chan struct{}      // channel to gracefully close a router
}

// RouteHandler handler function per tunnel that routes a message from one tunnel to another
type RouteHandler func(message *Message, tunnel *Tunnel, router *Router)

var routerList map[string]*Router
var routermux sync.Mutex

const (
	// Denotes max no of client can join a channel
	MAX_TUNNEL_PER_ROUTER = 10
)

func InitRouter() {
	routerList = make(map[string]*Router)
}

// CreateRouter create a router and add for a id
func CreateRouter(channelId string) *Router {
	router := &Router{
		ChannelId:    channelId,
		tunnels:      make([]*Tunnel, MAX_TUNNEL_PER_ROUTER),
		mapping:      make(map[string]*Tunnel),
		activeTunnel: 0,
		RouterDone:   make(chan struct{}),
	}
	// Create all tunnel at once
	for pos, _ := range router.tunnels {
		router.tunnels[pos] = GetTunnel()
	}
	routermux.Lock()
	routerList[channelId] = router
	routermux.Unlock()
	return router
}

// DeleteRouter delete router for a channel
func DeleteRouter(channelId string) {
	routermux.Lock()
	delete(routerList, channelId)
	routermux.Unlock()
}

// GetRouter get router for a channel
func GetRouter(channelId string) (*Router, error) {
	routermux.Lock()
	router, ok := routerList[channelId]
	routermux.Unlock()
	if !ok {
		return nil, fmt.Errorf("Router not found")
	}
	return router, nil
}

// RetriveFreeTunnel retrive a tunnel that is free
func (router *Router) RetriveFreeTunnel(clientId string) *Tunnel {
	router.tunnelsMux.Lock()
	for _, tunnel := range router.tunnels {
		if !tunnel.Accquired {
			tunnel.Accquired = true
			router.mapping[clientId] = tunnel
			router.activeTunnel = router.activeTunnel + 1
			log.Printf("retrived tunnel for serving %s", clientId)
			router.tunnelsMux.Unlock()
			return tunnel
		}
	}
	router.tunnelsMux.Unlock()
	return nil
}

// RetriveActiveClients retrive a list of active clients excluding the one specified
func (router *Router) RetriveActiveClients(exclude string) []string {
	clients := make([]string, 0)
	router.tunnelsMux.Lock()
	for client, _ := range router.mapping {
		if client != exclude {
			clients = append(clients, client)
		}
	}
	router.tunnelsMux.Unlock()
	return clients
}

//DefaultRouting handles the routing in server by forwarding request to the dst
func DefaultRouting(message *Message, tunnel *Tunnel, router *Router) {
	router.tunnelsMux.Lock()
	// Find tunnel based on destination specified
	dstTunnel, ok := router.mapping[message.DST]
	if !ok {
		log.Printf("Routing Error, client %s not found", message.DST)
	} else {
		data, _ := message.Marshal()
		// Send data out to the destination channel
		dstTunnel.Outgoing <- data

		log.Printf("routing signal (%s) from %s to %s", message.TYP, message.SRC, message.DST)
	}
	router.tunnelsMux.Unlock()
}

// ServeTunnel serve a tunnel in a router
func (router *Router) ServeTunnel(tunnel *Tunnel, handler RouteHandler) {
	tunnel.tunnelDone = make(chan struct{})
	go func() {
		for {
			select {
			case data := <-tunnel.Incoming:
				message, err := UnmarshalMessage(data)
				if err != nil {
					log.Printf("Error while handling message, %v\n", err)
					continue
				}
				switch {

				case message.TYP == SDP_DISCOVER_QUERY:

					log.Printf("handling peer discovery query")

					// handle discover query
					clients := router.RetriveActiveClients(message.SRC)
					discoverAnswer := CreateDiscoverAnswer(clients, message.SRC)
					data, _ := discoverAnswer.Marshal()
					tunnel.Outgoing <- data

				case message.TYP == SDP_DISCOVER_ANSWER:
					log.Printf("Dropping Unexpected DISCOVER_ANS")

				default:
					if message.DST == "" {
						log.Printf("Destination is not mentioned in SDP")
					}
					handler(message, tunnel, router)
				}
			case <-router.RouterDone:
				close(tunnel.tunnelDone)
				return
			case <-tunnel.tunnelDone:
				return
			}
		}
	}()
}

// DecommissionTunnel Decommision a tunnel
func (router *Router) DecommissionTunnel(tunnel *Tunnel, client string) {
	router.tunnelsMux.Lock()
	close(tunnel.tunnelDone)
	tunnel.Accquired = false
	router.activeTunnel = router.activeTunnel - 1
	delete(router.mapping, client)
	log.Printf("decommissioned tunnel form serving %s", client)
	router.tunnelsMux.Unlock()
}

// Close close a router
func (router *Router) Close() {
	fmt.Println("Stopping Router")
	close(router.RouterDone)
}
