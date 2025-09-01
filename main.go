package main

import (
	"bufio"
	"fmt"
	"log"
	"MyLocalBoard/internal/net"
	"MyLocalBoard/internal/ui"
	gonet "net"
	"os"
	"strings"
)

const (
	CustomURLScheme = "localboard://"
	Port            = 8888
)

func main() {
	args := os.Args
	if len(args) > 1 && strings.HasPrefix(args[1], CustomURLScheme) {
		runAsClient(args[1])
	} else {
		runAsHost()
	}
}

func runAsHost() {
	log.Println("Starting as HOST.")
	peerManager := net.NewPeerManager()
	go peerManager.StartTCPServer(Port)
	hostIP, err := net.GetOutgoingIP()
	if err != nil {
		log.Fatalf("Could not determine host IP: %v", err)
	}
	shareLink := fmt.Sprintf("%s%s:%d", CustomURLScheme, hostIP, Port)
	log.Printf("Share link: %s", shareLink)
	ui.RunApp(shareLink, nil)
}

func runAsClient(link string) {
	log.Printf("Starting as CLIENT, connecting to: %s", link)
	
	// --- THIS IS THE FIX ---
	address := strings.TrimPrefix(link, CustomURLScheme)
	// NEW LINE: Remove the trailing slash if the browser adds one.
	address = strings.TrimSuffix(address, "/") 

	conn, err := gonet.Dial("tcp", address)
	if err != nil {
		// We can leave the debug code here for now, just in case.
		log.Printf("FATAL: Could not connect to host at %s", address)
		log.Printf("THE ERROR IS: %v", err)
		log.Println("Press Enter to close this window...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		return
	}
	
	defer conn.Close()
	log.Printf("Successfully connected to host: %s", conn.RemoteAddr().String())
	ui.RunApp("", nil)
}