package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/rs/xid"

	"github.com/s8sg/satellite/pkg/transport"
)

type Server struct {
	Port int // Port to host the HTTP Server
}

// Serve
func (s *Server) Serve() error {

	transport.InitRouter()

	http.HandleFunc("/create", channelCreateHandler())
	http.HandleFunc("/tunnel", serveWS())

	if err := http.ListenAndServe(fmt.Sprintf(":%d", s.Port), nil); err != nil {
		return err
	}

	return nil
}

// channelCreateHandler
func channelCreateHandler() func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		// Generate channel ID
		channelID := xid.New().String()

		// Create Router
		transport.CreateRouter(channelID)

		// Return thr channel Id
		w.Write([]byte(channelID))
	}
}

// serveWS
func serveWS() func(w http.ResponseWriter, r *http.Request) {

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the channel and client id from header
		channel, cid := extractChannelId(r)
		if channel == "" {
			log.Println("Extraction failed, invalid channel or ID")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid channel or ID"))
			return
		}

		// Give back a router item based on the Channel ID
		router, err := transport.GetRouter(channel)
		if err != nil {
			log.Println("No Router found, invalid channel")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("invalid channel"))
			return
		}

		// Get the incoming and outgoing channel from/to the router
		tunnel := router.RetriveFreeTunnel(cid)
		if tunnel == nil {
			err = fmt.Errorf("Max no of connection reached, limit %d", transport.MAX_TUNNEL_PER_ROUTER)
			log.Printf("%v", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}
		router.ServeTunnel(tunnel, transport.DefaultRouting)

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log.Println(err)
			}
			return
		}

		log.Printf("SDP signal tunnel established with %s as %s for channel %s",
			cid, ws.RemoteAddr(), channel)

		connectionDone := make(chan struct{})

		// Handle incoming signal from the websocket and send it to incoming tunnel
		go func() {
			defer close(connectionDone)
			for {
				messageType, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("Error read:", err)
					return
				}

				switch messageType {
				case websocket.TextMessage:
					log.Println("TextMessage: ", message)
				case websocket.BinaryMessage:
					tunnel.Incoming <- message
				}
			}
		}()

		// Handle outgoing signal from the tunnel and send it to the websocket
		go func() {
			defer close(connectionDone)
			for {
				message := <-tunnel.Outgoing

				err := ws.WriteMessage(websocket.BinaryMessage, message)
				if err != nil {
					log.Println("Error write:", err)
					return
				}
			}

		}()

		<-connectionDone
		ws.Close()
		router.DecommissionTunnel(tunnel, cid)
	}
}

// extractChannelId
func extractChannelId(r *http.Request) (channel string, ID string) {
	auth := r.Header.Get("Authorization")
	prefix := "Bearer "
	if strings.HasPrefix(auth, prefix); len(auth) > len(prefix) {
		splits := strings.Split(auth[len(prefix):], "-")
		channel = splits[0]
		ID = splits[1]
	}
	return
}
