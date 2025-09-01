package main

import (
	"flag"
	"fmt"
	"log"

	"MyLocalBoard/internal/net"
	"MyLocalBoard/internal/state"
	"MyLocalBoard/internal/ui"
)

func main() {
	asHost := flag.Bool("host", false, "Run as host (advertise session)")
	join := flag.Bool("join", true, "Auto-discover & join session on LAN")
	port := flag.Int("port", 8080, "TCP port")
	name := flag.String("name", "", "Your display name (optional)")
	flag.Parse()

	// Shared replicated state (CRDT)
	s := state.NewBoardState()

	// Networking: TCP + mDNS (host optionally advertises)
	tr, err := net.NewTransport(*port, *asHost)
	if err != nil {
		log.Fatal(err)
	}
	defer tr.Close()

	// Wire the transport to replicate CRDT ops
	tr.OnRemoteOp = func(op state.Op) {
		// integrate into CRDT and notify UI
		if s.Apply(op) {
			ui.InvalidateCanvas() // redraw
		}
	}

	// On local ops, broadcast to peers
	state.OnLocalOp = func(op state.Op) {
		tr.Broadcast(op)
	}

	// Optionally start discovery & dial peers
	if *join {
		if err := tr.DiscoverAndConnect(); err != nil {
			log.Println("discovery:", err)
		}
	}

	// Start GUI
	ui.RunApp(ui.AppConfig{
		DisplayName: *name,
		State:       s,
		Transport:   tr,
	})
	fmt.Println("Goodbye.")
}
