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

	// "fyne.io/fyne/v2/app"
	// Import your own packages
	"MyLocalBoard/internal/ui"
)

const (
	CustomURLScheme = "localboard://"
	Port            = 8888
)

// NetworkMessage defines the structure for all network communication.
type NetworkMessage struct {
	Type string    `json:"type"`
	Path ui.Path `json:"path,omitempty"` // Use the Path struct from the ui package
}

// ConnectionManager handles active network connections.
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
	cm.mu.Lock(); defer cm.mu.Unlock()
	cm.connections[conn] = true
	log.Printf("Added connection: %s", conn.RemoteAddr().String())
}
func (cm *ConnectionManager) Remove(conn net.Conn) {
	cm.mu.Lock(); defer cm.mu.Unlock()
	delete(cm.connections, conn)
	log.Printf("Removed connection: %s", conn.RemoteAddr().String())
}
func (cm *ConnectionManager) Broadcast(data []byte, exclude net.Conn) {
	cm.mu.RLock(); defer cm.mu.RUnlock()
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
	connManager := NewConnectionManager()

	// Connect the board's drawing event to the network
	board.OnNewPath = func(p ui.Path) {
		msg := NetworkMessage{Type: "draw", Path: p}
		data, _ := json.Marshal(msg)
		log.Printf("[HOST] Broadcasting path with %d points", len(p.Points))
		connManager.Broadcast(data, nil)
	}

	// Start the TCP server to listen for clients
	go startHostServer(connManager, board)

	// Get the share link and run the UI
	hostIP := getLocalIP()
	shareLink := fmt.Sprintf("%s%s:%d", CustomURLScheme, hostIP, Port)
	ui.RunApp(shareLink, board)
}

func startHostServer(connManager *ConnectionManager, board *ui.BoardWidget) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil { log.Fatalf("Failed to start server: %v", err) }
	defer listener.Close()
	log.Printf("Host server listening on port %d", Port)

	for {
		conn, err := listener.Accept()
		if err != nil { continue }
		connManager.Add(conn)
		go handleHostConnection(conn, connManager, board)
	}
}

func handleHostConnection(conn net.Conn, connManager *ConnectionManager, board *ui.BoardWidget) {
	defer conn.Close(); defer connManager.Remove(conn)
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
			board.AddRemotePath(msg.Path) // This call is now safe
			data, _ := json.Marshal(msg)
			connManager.Broadcast(data, conn) // Relay to other clients
		}
	}
}

func runClient(link string) {
	log.Println("Starting as CLIENT")
	board := ui.NewBoardWidget()

	// We start the UI immediately and connect in the background
	go connectToHost(link, board)

	ui.RunApp("", board)
}

func connectToHost(link string, board *ui.BoardWidget) {
	address := strings.TrimPrefix(link, CustomURLScheme)
	address = strings.TrimSuffix(address, "/")

	// Give the UI a moment to appear before trying to connect
	time.Sleep(500 * time.Millisecond)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		board.SetStatus("Connection failed!") // Update status via the board's safe method
		return
	}
	defer conn.Close()
	log.Println("Client connected successfully")
	board.SetStatus("Connected to host")

	// Now that we're connected, set up the drawing callback
	board.OnNewPath = func(p ui.Path) {
		msg := NetworkMessage{Type: "draw", Path: p}
		encoder := json.NewEncoder(conn)
		if err := encoder.Encode(msg); err != nil {
			log.Printf("Failed to send drawing: %v", err)
		}
	}

	// Listen for incoming drawings from the host
	decoder := json.NewDecoder(conn)
	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Disconnected from host: %v", err)
			board.SetStatus("Disconnected from host")
			return
		}

		if msg.Type == "draw" {
			board.AddRemotePath(msg.Path) // This is safe because of the channel in BoardWidget
		}
	}
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80"); if err != nil { return "127.0.0.1" }; defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr); return localAddr.IP.String()
}