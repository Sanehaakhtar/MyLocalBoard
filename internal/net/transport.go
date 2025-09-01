package net

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// Peer represents a connected client to the host.
type Peer struct {
	Conn net.Conn
}

// PeerManager is used by the HOST to manage all active client connections.
type PeerManager struct {
	peers map[string]*Peer
	mu    sync.RWMutex
}

// NewPeerManager creates a new manager.
func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[string]*Peer),
	}
}

// Add adds a new peer (a client that just connected) to the manager.
func (pm *PeerManager) Add(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	addr := peer.Conn.RemoteAddr().String()
	pm.peers[addr] = peer
	// This log message is our confirmation on the Host side!
	log.Printf("CONFIRMED: A client has connected from %s", addr)
}

// StartTCPServer is run by the HOST to listen for incoming connections from clients.
func (pm *PeerManager) StartTCPServer(port int) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start TCP server on port %d: %v", port, err)
	}
	defer listener.Close()
	log.Printf("TCP host server listening on port %d...", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		pm.Add(&Peer{Conn: conn})
		// We are not handling communication yet, just accepting the connection.
	}
}