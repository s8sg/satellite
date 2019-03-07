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
		router := transport.CreateRouter(channelID)
		router.Serve()

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
		channelId := extractChannelId(r)
		if channelId == "" {
			log.Println("Extraction failed, invalid channel ID")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid channel ID"))
			return
		}

		// Give back a router item based on the Channel ID
		router, err := transport.GetRouter(channelId)
		if err != nil {
			log.Println("No Router found, invalid channel ID")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("invalid channel ID"))
			return
		}

		// Get the incoming and outgioing channel from/to the router
		incoming, outgoing := router.RetriveClientChannel()

		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log.Println(err)
			}
			return
		}

		log.Printf("SDP channel %s established with: %s",
			channelId, ws.RemoteAddr())

		connectionDone := make(chan struct{})

		// Handle incoming
		go func() {
			defer close(connectionDone)
			for {
				messageType, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("read:", err)
					return
				}

				switch messageType {
				case websocket.TextMessage:
					log.Println("TextMessage: ", message)
				case websocket.BinaryMessage:
					incoming <- message
				}
			}
		}()

		// Handle outgoing
		go func() {
			defer close(connectionDone)
			for {
				message := <-outgoing

				err := ws.WriteMessage(websocket.BinaryMessage, message)
				if err != nil {
					log.Println("write:", err)
					return
				}
			}

		}()

		// close connection to server once exchnage is done
		<-connectionDone
		ws.Close()
		router.Close()
		transport.DeleteRouter(channelId)
	}
}

// extractChannelId
func extractChannelId(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	prefix := "Bearer "
	if strings.HasPrefix(auth, prefix); len(auth) > len(prefix) {
		return auth[len(prefix):]
	}
	return ""
}
