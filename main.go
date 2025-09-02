package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"MyLocalBoard/internal/ui"
)

const (
	CustomURLScheme = "localboard://"
	Port            = 8888
)

// NetworkMessage now supports a "clear" operation with an OwnerID
type NetworkMessage struct {
	Type    string  `json:"type"`
	Path    ui.Path `json:"path,omitempty"`
	OwnerID string  `json:"owner_id,omitempty"` // Used for the "clear" message
}

// ConnectionManager handles active network connections
type ConnectionManager struct {
	connections map[net.Conn]bool
	mu          sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[net.Conn]bool),
	}
}

func (cm *ConnectionManager) Add(conn net.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.connections[conn] = true
	log.Printf("Added connection: %s", conn.RemoteAddr().String())
}

func (cm *ConnectionManager) Remove(conn net.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.connections, conn)
	log.Printf("Removed connection: %s", conn.RemoteAddr().String())
}

func (cm *ConnectionManager) Broadcast(data []byte, exclude net.Conn) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	dataWithNewline := append(data, '\n')
	for conn := range cm.connections {
		if conn != exclude {
			if _, err := conn.Write(dataWithNewline); err != nil {
				log.Printf("Error sending to %s: %v", conn.RemoteAddr().String(), err)
			}
		}
	}
}

func main() {
	args := os.Args
	if len(args) > 1 && strings.HasPrefix(args[1], CustomURLScheme) {
		runClient(args[1])
	} else {
		runHost()
	}
}

func runHost() {
	log.Println("Starting as HOST")
	board := ui.NewBoardWidget()
	board.SetLocalClientID("host") // The host identifies itself with a special ID
	connManager := NewConnectionManager()

	// When the host draws a path
	board.OnNewPath = func(p ui.Path) {
		board.AddRemotePath(p) // Draw it locally first
		msg := NetworkMessage{Type: "draw", Path: p}
		data, _ := json.Marshal(msg)
		connManager.Broadcast(data, nil)
	}

	// When the host clicks its "Clear" button
	board.OnClear = func() {
		log.Println("[HOST] Broadcasting clear message for self (owner: host).")
		msg := NetworkMessage{Type: "clear", OwnerID: "host"}
		data, _ := json.Marshal(msg)
		connManager.Broadcast(data, nil)
	}

	go startHostServer(connManager, board)

	hostIP := getLocalIP()
	shareLink := fmt.Sprintf("%s%s:%d", CustomURLScheme, hostIP, Port)
	ui.RunApp(shareLink, board)
}

func startHostServer(connManager *ConnectionManager, board *ui.BoardWidget) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	log.Printf("Host server listening on port %d", Port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		connManager.Add(conn)
		go handleHostConnection(conn, connManager, board)
	}
}

func handleHostConnection(conn net.Conn, connManager *ConnectionManager, board *ui.BoardWidget) {
	defer conn.Close()
	defer connManager.Remove(conn)
	addr := conn.RemoteAddr().String()
	decoder := json.NewDecoder(conn)

	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Client %s disconnected: %v", addr, err)
			return
		}

		log.Printf("[HOST] Received '%s' from %s", msg.Type, addr)
		if msg.Type == "draw" {
			// Trust the client's path and relay it
			board.AddRemotePath(msg.Path)
			data, _ := json.Marshal(msg)
			connManager.Broadcast(data, conn) // Relay to OTHERS
		} else if msg.Type == "clear" {
			// Trust the client's clear command and relay it
			board.ClearRemote(msg.OwnerID)
			data, _ := json.Marshal(msg)
			connManager.Broadcast(data, conn) // Relay to OTHERS
		}
	}
}

func runClient(link string) {
	log.Println("Starting as CLIENT")
	board := ui.NewBoardWidget()
	// The client's ID will be set once it connects and knows its address
	go connectToHost(link, board)
	ui.RunApp("", board)
}

func connectToHost(link string, board *ui.BoardWidget) {
	address := strings.TrimPrefix(link, CustomURLScheme)
	address = strings.TrimSuffix(address, "/")
	time.Sleep(500 * time.Millisecond) // Give UI time to launch

	conn, err := net.Dial("tcp", address)
	if err != nil {
		board.SetStatus(fmt.Sprintf("Connection failed: %v", err))
		return
	}
	defer conn.Close()

	// A client's unique ID is its full network address (IP:Port)
	localAddr := conn.LocalAddr().String()
	board.SetLocalClientID(localAddr)
	board.SetStatus("Connected to host as " + localAddr)
	log.Println("Client connected successfully as", localAddr)

	encoder := json.NewEncoder(conn)

	// When the client draws, it sends its drawing to the host
	board.OnNewPath = func(p ui.Path) {
		msg := NetworkMessage{Type: "draw", Path: p}
		if err := encoder.Encode(msg); err != nil {
			log.Printf("Failed to send drawing: %v", err)
		}
	}

	// When the client clears, it sends a clear command with its own ID
	board.OnClear = func() {
		msg := NetworkMessage{Type: "clear", OwnerID: board.LocalClientID}
		if err := encoder.Encode(msg); err != nil {
			log.Printf("Failed to send clear message: %v", err)
		}
	}

	// Listen for incoming messages from the host
	decoder := json.NewDecoder(conn)
	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			board.SetStatus(fmt.Sprintf("Disconnected from host: %v", err))
			return
		}

		if msg.Type == "draw" {
			// Don't draw our own path again, the UI handles that.
			// Only draw paths from OTHER users.
			if msg.Path.OwnerID != board.LocalClientID {
				board.AddRemotePath(msg.Path)
			}
		} else if msg.Type == "clear" {
			// Everyone, including the original sender, clears based on the broadcast.
			board.ClearRemote(msg.OwnerID)
		}
	}
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1" // Fallback
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}