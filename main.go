package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/rs/xid"
	"os"
	"os/signal"

	client "github.com/s8sg/satellite/pkg/client"
	server "github.com/s8sg/satellite/pkg/server"
	transport "github.com/s8sg/satellite/pkg/transport"
)

var (
	Version string
	Commit  string
)

// Argument passed from the command-line
type Args struct {
	Port    int
	UIPort  int
	Remote  string
	Version bool
	Create  bool
	Join    string
}

func main() {
	args := Args{}
	flag.BoolVar(&args.Version, "version", false, "print version information and exit")
	flag.IntVar(&args.Port, "port", 8000, "port for server or client's ui")
	flag.BoolVar(&args.Create, "create", false, "create new channel")
	flag.StringVar(&args.Join, "join", "", "channel id")
	flag.StringVar(&args.Remote, "remote", "127.0.0.1:8000", " server address i.e. 127.0.0.1:8000")
	flag.Parse()

	isServer := args.Join == "" && !args.Create

	switch {
	case args.Version:
		PrintVersionInfo()
		os.Exit(0)

	case isServer:
		// Server mode
		server := server.Server{
			Port: args.Port,
		}
		err := server.Serve()
		if err != nil {
			panic(err)
		}

	case !isServer:
		// Client Mode
		client := client.Client{
			Remote:   args.Remote,
			Port:     args.Port,
			ID:       xid.New().String(),
			IOTunnel: transport.GetTunnel(),
		}
		if args.Create {
			err := client.CreateChannel()
			if err != nil {
				panic(err)
			}
		} else if args.Join != "" {
			client.Channel = args.Join
			err := client.JoinChannel()
			if err != nil {
				panic(err)
			}
		} else {
			flag.Usage()
		}
		go func() {
			for {
				reader := bufio.NewReader(os.Stdin)
				message, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				// Send to Outgoing to broadcast
				client.IOTunnel.Outgoing <- []byte(message)
			}
		}()

		go func() {
			for {
				// Print incoming message
				message := <-client.IOTunnel.Incoming
				fmt.Println(string(message))
			}
		}()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)

		<-c
		fmt.Println("Caught signal, stopping")

	}

}

func PrintVersionInfo() {
	if len(Version) == 0 {
		fmt.Println("Version: dev")
	} else {
		fmt.Println("Version:", Version)
	}
	fmt.Println("Git Commit:", Commit)
}
