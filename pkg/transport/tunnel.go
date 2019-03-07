package transport

import (
	"fmt"
	"sync"
	//	"time"
)

type Tunnel struct {
	Incoming chan []byte
	Outgoing chan []byte
}

func GetTunnel() *Tunnel {
	t := &Tunnel{}
	t.Incoming = make(chan []byte)
	t.Outgoing = make(chan []byte)
	return t
}

func (t *Tunnel) Close() {
	close(t.Incoming)
	close(t.Outgoing)
}

type Router struct {
	ChannelId string

	Client1Incoming chan []byte
	Client1Outgoing chan []byte

	Client2Incoming chan []byte
	Client2Outgoing chan []byte

	client    int
	clientMux sync.Mutex

	RouterDone chan struct{}
}

var routerList map[string]*Router
var mux sync.Mutex

func InitRouter() {
	routerList = make(map[string]*Router)
}

func CreateRouter(channelId string) *Router {
	router := &Router{
		ChannelId:       channelId,
		Client1Incoming: make(chan []byte),
		Client2Outgoing: make(chan []byte),
		Client1Outgoing: make(chan []byte),
		Client2Incoming: make(chan []byte),
		client:          0,
		RouterDone:      make(chan struct{}),
	}
	mux.Lock()
	routerList[channelId] = router
	mux.Unlock()
	return router
}

func DeleteRouter(channelId string) {
	mux.Lock()
	delete(routerList, channelId)
	mux.Unlock()
}

func GetRouter(channelId string) (*Router, error) {
	mux.Lock()
	router, ok := routerList[channelId]
	mux.Unlock()
	if !ok {
		return nil, fmt.Errorf("Router not found")
	}
	return router, nil
}

// RetriveClientId retrive the client ID and update on retrive
func (router *Router) RetriveClientChannel() (incoming chan []byte, outgoing chan []byte) {
	router.clientMux.Lock()
	id := router.client
	router.client += 1
	router.clientMux.Unlock()
	switch id {
	case 0:
		incoming = router.Client1Incoming
		outgoing = router.Client1Outgoing
	case 1:
		incoming = router.Client2Incoming
		outgoing = router.Client2Outgoing
	}
	return
}

// Serve transfer
func (router *Router) Serve() {
	router.RouterDone = make(chan struct{})
	go func() {
		for {
			select {
			case data := <-router.Client2Incoming:
				router.Client1Outgoing <- data
			case data := <-router.Client1Incoming:
				router.Client2Outgoing <- data
			case <-router.RouterDone:
				fmt.Println("Stopping Router")
				return
			}
		}
	}()
}

// Close close a router
func (router *Router) Close() {
	close(router.RouterDone)
	close(router.Client1Outgoing)
	close(router.Client2Outgoing)
	close(router.Client1Incoming)
	close(router.Client2Incoming)
}
