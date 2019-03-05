package main

import (
	"flag"
	"fmt"
	"os"

	client "github.com/s8sg/joinin/pkg/client"
	server "github.com/s8sg/joinin/pkg/server"
)

var (
	Version string
	Commit  string
)

// Argument passed from the command-line
type Args struct {
	Port    int
	Server  bool
	Remote  string
	Version bool
}

func main() {
	args := Args{}
	flag.BoolVar(&args.Version, "version", false, "print version information and exit")
	flag.IntVar(&args.Port, "port", 8000, "port for server")
	flag.BoolVar(&args.Server, "server", true, "server or client")
	flag.StringVar(&args.Remote, "remote", "127.0.0.1:8000", " server address i.e. 127.0.0.1:8000")
	flag.Parse()

	switch {
	case args.Version:
		PrintVersionInfo()
		os.Exit(0)

	case args.Server:
		server := server.Server{
			Port: args.Port,
		}
		err := server.Serve()
		if err != nil {
			panic(err)
		}

	case !args.Server:
		client := client.Client{
			Remote: args.Remote,
		}
		err := client.Connect()
		if err != nil {
			panic(err)
		}
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
