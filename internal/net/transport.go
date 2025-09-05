package net

import (
	"fmt"
	"log"
	"net"
	"sync"

	"fyne.io/fyne/v2"
)

// DrawingArea represents a rectangular area on the canvas
type DrawingArea struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Path represents a drawing path
type Path struct {
	ID     string          `json:"id"`
	Points []fyne.Position `json:"points"`
	Color  string          `json:"color"`
	Stroke float32         `json:"stroke"`
}

// Message struct to wrap data with client identification
type Message struct {
	Data     []byte
	ClientID string
}

// Operation represents all types of operations that can be sent
type Operation struct {
	Type          string      `json:"type"` // "draw", "request_permission", "permission_response"
	Path          *Path       `json:"path,omitempty"`
	RequestedArea DrawingArea `json:"requested_area,omitempty"`
	AssignedArea  *DrawingArea `json:"assigned_area,omitempty"`
	Granted       bool        `json:"granted,omitempty"`
	ClientID      string      `json:"client_id,omitempty"`
}

// DrawOperation kept for backward compatibility
type DrawOperation struct {
	Path Path `json:"path"`
}

type Peer struct {
	Conn     net.Conn
	ClientID string
}

type PeerManager struct {
	peers    map[string]*Peer
	mu       sync.RWMutex
	Messages chan Message // Now sends Message struct with client ID
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers:    make(map[string]*Peer),
		Messages: make(chan Message, 100), // Buffered channel
	}
}

func (pm *PeerManager) Add(peer *Peer) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	addr := peer.Conn.RemoteAddr().String()
	peer.ClientID = addr // Use address as client ID for now
	pm.peers[addr] = peer
	log.Printf("Added new client connection: %s", addr)
}

func (pm *PeerManager) Remove(clientID string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if peer, exists := pm.peers[clientID]; exists {
		peer.Conn.Close()
		delete(pm.peers, clientID)
		log.Printf("Removed client connection: %s", clientID)
	}
}

func (pm *PeerManager) Broadcast(msg []byte) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	// Add newline delimiter for proper message framing
	msgWithDelim := append(msg, '\n')
	
	for addr, peer := range pm.peers {
		_, err := peer.Conn.Write(msgWithDelim)
		if err != nil {
			log.Printf("Error writing to client %s: %v. Removing client.", addr, err)
			go pm.Remove(addr) // Remove in goroutine to avoid deadlock
		}
	}
}

func (pm *PeerManager) BroadcastExcept(excludeClientID string, msg []byte) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	msgWithDelim := append(msg, '\n')
	
	for addr, peer := range pm.peers {
		if addr != excludeClientID {
			_, err := peer.Conn.Write(msgWithDelim)
			if err != nil {
				log.Printf("Error writing to client %s: %v. Removing client.", addr, err)
				go pm.Remove(addr)
			}
		}
	}
}

func (pm *PeerManager) SendToClient(clientID string, msg []byte) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	if peer, exists := pm.peers[clientID]; exists {
		msgWithDelim := append(msg, '\n')
		_, err := peer.Conn.Write(msgWithDelim)
		if err != nil {
			log.Printf("Error writing to client %s: %v. Removing client.", clientID, err)
			go pm.Remove(clientID)
		}
	}
}

func (pm *PeerManager) StartTCPServer(port int) {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start TCP server on port %d: %v", port, err)
	}
	defer listener.Close()
	log.Printf("TCP host server listening on port %d", port)
	
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		peer := &Peer{Conn: conn}
		pm.Add(peer)
		go pm.handleConnection(peer)
	}
}

func (pm *PeerManager) handleConnection(peer *Peer) {
	addr := peer.Conn.RemoteAddr().String()
	defer func() {
		log.Printf("Client %s disconnected", addr)
		pm.Remove(addr)
	}()
	
	buffer := make([]byte, 0)
	tempBuf := make([]byte, 4096)
	
	for {
		n, err := peer.Conn.Read(tempBuf)
		if err != nil {
			break
		}
		
		log.Printf("[HOST NETWORK] Received %d bytes from client %s", n, addr)
		buffer = append(buffer, tempBuf[:n]...)
		
		// Process complete messages (delimited by newlines)
		for {
			newlineIdx := -1
			for i, b := range buffer {
				if b == '\n' {
					newlineIdx = i
					break
				}
			}
			
			if newlineIdx == -1 {
				break // No complete message yet
			}
			
			messageBytes := buffer[:newlineIdx]
			buffer = buffer[newlineIdx+1:]
			
			if len(messageBytes) > 0 {
				msg := Message{
					Data:     messageBytes,
					ClientID: addr,
				}
				
				select {
				case pm.Messages <- msg:
				default:
					log.Println("Message channel full, dropping message")
				}
			}
		}
	}
}