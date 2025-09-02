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

// NetworkMessage now supports multiple message types including state sync
type NetworkMessage struct {
	Type     string    `json:"type"`     // "draw", "clear", "sync_state", "request_sync"
	Path     ui.Path   `json:"path,omitempty"`
	Paths    []ui.Path `json:"paths,omitempty"`  // For sending multiple paths at once
	OwnerID  string    `json:"owner_id,omitempty"`
	ClientID string    `json:"client_id,omitempty"`
}

// ConnectionManager handles active network connections
type ConnectionManager struct {
	connections map[net.Conn]string // conn -> clientID
	mu          sync.RWMutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[net.Conn]string),
	}
}

func (cm *ConnectionManager) Add(conn net.Conn) string {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	clientID := conn.RemoteAddr().String()
	cm.connections[conn] = clientID
	log.Printf("Added connection: %s", clientID)
	return clientID
}

func (cm *ConnectionManager) Remove(conn net.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if clientID, exists := cm.connections[conn]; exists {
		delete(cm.connections, conn)
		log.Printf("Removed connection: %s", clientID)
	}
}

func (cm *ConnectionManager) Broadcast(data []byte, exclude net.Conn) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	dataWithNewline := append(data, '\n')
	for conn := range cm.connections {
		if conn != exclude {
			if _, err := conn.Write(dataWithNewline); err != nil {
				log.Printf("Error sending to %s: %v", cm.connections[conn], err)
			}
		}
	}
}

func (cm *ConnectionManager) SendToConnection(conn net.Conn, data []byte) {
	dataWithNewline := append(data, '\n')
	if _, err := conn.Write(dataWithNewline); err != nil {
		log.Printf("Error sending to %s: %v", conn.RemoteAddr().String(), err)
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
	board.SetLocalClientID("host")
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
		
		// Send current state to the new client immediately
		go func() {
			time.Sleep(100 * time.Millisecond) // Small delay to ensure connection is ready
			sendCurrentStateToClient(conn, board)
		}()
		
		go handleHostConnection(conn, connManager, board)
	}
}

func sendCurrentStateToClient(conn net.Conn, board *ui.BoardWidget) {
	// Get all current paths from the board
	currentPaths := board.GetAllPaths()
	
	if len(currentPaths) > 0 {
		log.Printf("[HOST] Sending %d existing paths to new client %s", len(currentPaths), conn.RemoteAddr().String())
		
		// Send state sync message with all current paths
		msg := NetworkMessage{
			Type:  "sync_state",
			Paths: currentPaths,
		}
		
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling sync state: %v", err)
			return
		}
		
		dataWithNewline := append(data, '\n')
		if _, err := conn.Write(dataWithNewline); err != nil {
			log.Printf("Error sending sync state to %s: %v", conn.RemoteAddr().String(), err)
		}
	} else {
		log.Printf("[HOST] No existing paths to sync for new client %s", conn.RemoteAddr().String())
	}
}

func handleHostConnection(conn net.Conn, connManager *ConnectionManager, board *ui.BoardWidget) {
	defer conn.Close()
	defer connManager.Remove(conn)
	addr := conn.RemoteAddr().String()
	
	buffer := make([]byte, 0)
	tempBuf := make([]byte, 4096)

	for {
		n, err := conn.Read(tempBuf)
		if err != nil {
			log.Printf("Client %s disconnected: %v", addr, err)
			return
		}
		
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
				var msg NetworkMessage
				if err := json.Unmarshal(messageBytes, &msg); err != nil {
					log.Printf("Error unmarshaling message from %s: %v", addr, err)
					continue
				}

				log.Printf("[HOST] Received '%s' from %s", msg.Type, addr)
				
				switch msg.Type {
				case "draw":
					// Trust the client's path and relay it
					board.AddRemotePath(msg.Path)
					data, _ := json.Marshal(msg)
					connManager.Broadcast(data, conn) // Relay to OTHERS
					
				case "clear":
					// Trust the client's clear command and relay it
					board.ClearRemote(msg.OwnerID)
					data, _ := json.Marshal(msg)
					connManager.Broadcast(data, conn) // Relay to OTHERS
					
				case "request_sync":
					// Client is requesting current state
					go sendCurrentStateToClient(conn, board)
				}
			}
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

	// When the client draws, it sends its drawing to the host
	board.OnNewPath = func(p ui.Path) {
    msg := NetworkMessage{Type: "draw", Path: p}
    data, _ := json.Marshal(msg)
    dataWithNewline := append(data, '\n')
    if _, err := conn.Write(dataWithNewline); err != nil {
        log.Printf("Failed to send drawing: %v", err)
    }
}
	// When the client clears, it sends a clear command with its own ID
board.OnClear = func() {
    msg := NetworkMessage{Type: "clear", OwnerID: board.LocalClientID}
    data, _ := json.Marshal(msg)
    dataWithNewline := append(data, '\n')
    if _, err := conn.Write(dataWithNewline); err != nil {
        log.Printf("Failed to send clear message: %v", err)
    }
}

	// Optional: Request current state from host
	// Uncomment this if you want clients to explicitly request sync
	/*
	requestSync := NetworkMessage{Type: "request_sync", ClientID: localAddr}
	data, _ := json.Marshal(requestSync)
	dataWithNewline := append(data, '\n')
	conn.Write(dataWithNewline)
	*/

	// Listen for incoming messages from the host
	buffer := make([]byte, 0)
	tempBuf := make([]byte, 4096)
	
	for {
		n, err := conn.Read(tempBuf)
		if err != nil {
			board.SetStatus(fmt.Sprintf("Disconnected from host: %v", err))
			return
		}
		
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
				var msg NetworkMessage
				if err := json.Unmarshal(messageBytes, &msg); err != nil {
					log.Printf("Error unmarshaling message: %v", err)
					continue
				}

				switch msg.Type {
				case "draw":
					// Don't draw our own path again, the UI handles that.
					// Only draw paths from OTHER users.
					if msg.Path.OwnerID != board.LocalClientID {
						board.AddRemotePath(msg.Path)
					}
					
				case "clear":
					// Everyone, including the original sender, clears based on the broadcast.
					board.ClearRemote(msg.OwnerID)
					
				case "sync_state":
					// Receive all existing paths from host
					log.Printf("[CLIENT] Received state sync with %d paths", len(msg.Paths))
					for _, path := range msg.Paths {
						if path.OwnerID != board.LocalClientID {
							board.AddRemotePath(path)
						}
					}
				}
			}
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