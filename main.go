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

// --- Structs and Constants ---
const (
    CustomURLScheme = "localboard://"
    Port            = 8888
)

type NetworkMessage struct {
    Type    string    `json:"type"`
    Path    ui.Path   `json:"path,omitempty"`
    Paths   []ui.Path `json:"paths,omitempty"`
    OwnerID string    `json:"owner_id,omitempty"`
}

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
                log.Printf("Error sending message: %v", err)
            }
        }
    }
}

func getLocalIP() string {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        return "127.0.0.1"
    }
    defer conn.Close()
    localAddr := conn.LocalAddr().(*net.UDPAddr)
    return localAddr.IP.String()
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
	
	board.OnNewPath = func(p ui.Path) {
		log.Printf("Host: New path with %d points", len(p.Points))
		board.AddRemotePath(p) // Draw locally
		msg := NetworkMessage{Type: "draw", Path: p}
		data, _ := json.Marshal(msg)
		connManager.Broadcast(data, nil)
	}
	
	board.OnClear = func() {
		log.Println("Host: Clearing paths")
		board.ClearRemote(board.LocalClientID) // Clear locally
		msg := NetworkMessage{Type: "clear", OwnerID: board.LocalClientID}
		data, _ := json.Marshal(msg)
		connManager.Broadcast(data, nil)
	}
	
	board.OnSave = func() []ui.Path {
		paths := board.GetAllPathsAsValues()
		log.Printf("Host: Saving %d paths", len(paths))
		return paths
	}
	
	board.OnLoad = func(paths []ui.Path) {
		log.Printf("Host: Loading %d paths and broadcasting to clients", len(paths))
		
		// Broadcast to clients in a goroutine to avoid blocking
		go func() {
			loadMsg := NetworkMessage{Type: "sync_state", Paths: paths}
			loadData, err := json.Marshal(loadMsg)
			if err != nil {
				log.Printf("Error marshaling load message: %v", err)
				return
			}
			connManager.Broadcast(loadData, nil)
			log.Printf("Broadcasted %d paths to all clients", len(paths))
		}()
	}

	go startHostServer(connManager, board)
	hostIP := getLocalIP()
	shareLink := fmt.Sprintf("%s%s:%d", CustomURLScheme, hostIP, Port)
	log.Printf("Share link: %s", shareLink)
	ui.RunApp(shareLink, board)
}

func startHostServer(connManager *ConnectionManager, board *ui.BoardWidget) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
	if err != nil { 
		log.Fatalf("Server start failed: %v", err) 
	}
	defer listener.Close()
	
	log.Printf("Host server listening on port %d", Port)
	
	for {
		conn, err := listener.Accept()
		if err != nil { 
			log.Printf("Error accepting connection: %v", err)
			continue 
		}
		
		connManager.Add(conn)
		
		// Send current state to new client after a brief delay
		go func(c net.Conn) { 
			time.Sleep(100 * time.Millisecond)
			sendCurrentStateToClient(c, board) 
		}(conn)
		
		go handleHostConnection(conn, connManager, board)
	}
}

func sendCurrentStateToClient(conn net.Conn, board *ui.BoardWidget) {
	paths := board.GetAllPathsAsValues()
	if len(paths) > 0 {
		msg := NetworkMessage{Type: "sync_state", Paths: paths}
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("Error marshaling sync state: %v", err)
			return
		}
		
		if _, err := conn.Write(append(data, '\n')); err != nil { 
			log.Printf("Sync failed: %v", err) 
		} else {
			log.Printf("Sent %d paths to new client", len(paths))
		}
	}
}

func handleHostConnection(conn net.Conn, connManager *ConnectionManager, board *ui.BoardWidget) {
	defer conn.Close()
	defer connManager.Remove(conn)
	
	decoder := json.NewDecoder(conn)
	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil { 
			log.Printf("Connection closed or decode error: %v", err)
			return 
		}

		switch msg.Type {
		case "draw":
			log.Printf("Host received draw from client with %d points", len(msg.Path.Points))
			board.AddRemotePath(msg.Path)
			data, _ := json.Marshal(msg)
			connManager.Broadcast(data, conn)
		case "clear":
			log.Printf("Host received clear from client: %s", msg.OwnerID)
			board.ClearRemote(msg.OwnerID)
			data, _ := json.Marshal(msg)
			connManager.Broadcast(data, conn)
		default:
			log.Printf("Unknown message type from client: %s", msg.Type)
		}
	}
}

func runClient(link string) {
	log.Println("Starting as CLIENT")
	board := ui.NewBoardWidget()
	
	// Set up client-specific handlers
	board.OnSave = func() []ui.Path {
		paths := board.GetAllPathsAsValues()
		log.Printf("Client: Saving %d paths", len(paths))
		return paths
	}
	
	board.OnLoad = func(paths []ui.Path) {
		// For clients, just load locally - don't sync over network during load
		log.Printf("Client: Loading %d paths locally", len(paths))
	}
	
	go connectToHost(link, board)
	ui.RunApp("", board)
}

func connectToHost(link string, board *ui.BoardWidget) {
	address := strings.TrimPrefix(link, CustomURLScheme)
	address = strings.TrimSuffix(address, "/")
	
	log.Printf("Client connecting to: %s", address)
	board.SetStatus("Connecting to " + address + "...")
	time.Sleep(500 * time.Millisecond)
	
	conn, err := net.Dial("tcp", address)
	if err != nil { 
		board.SetStatus("Connection failed: " + err.Error())
		log.Printf("Connection failed: %v", err)
		return 
	}
	defer conn.Close()
	
	localAddr := conn.LocalAddr().String()
	board.SetLocalClientID(localAddr)
	board.SetStatus("Connected as " + localAddr)
	log.Println("Client connected as", localAddr)
	
	encoder := json.NewEncoder(conn)
	
	board.OnNewPath = func(p ui.Path) {
		log.Printf("Client: New path with %d points", len(p.Points))
		board.AddRemotePath(p) // Draw locally
		msg := NetworkMessage{Type: "draw", Path: p}
		if err := encoder.Encode(msg); err != nil {
			log.Printf("Error sending draw message: %v", err)
		}
	}
	
	board.OnClear = func() {
		log.Println("Client: Clearing paths")
		board.ClearRemote(board.LocalClientID) // Clear locally
		msg := NetworkMessage{Type: "clear", OwnerID: board.LocalClientID}
		if err := encoder.Encode(msg); err != nil {
			log.Printf("Error sending clear message: %v", err)
		}
	}

	decoder := json.NewDecoder(conn)
	for {
		var msg NetworkMessage
		if err := decoder.Decode(&msg); err != nil { 
			board.SetStatus("Disconnected: " + err.Error())
			log.Printf("Disconnected: %v", err)
			return 
		}
		
		switch msg.Type {
		case "draw":
			if msg.Path.OwnerID != board.LocalClientID { 
				log.Printf("Client: Received remote path with %d points", len(msg.Path.Points))
				board.AddRemotePath(msg.Path) 
			}
		case "clear":
			log.Printf("Client: Received clear for owner: %s", msg.OwnerID)
			board.ClearRemote(msg.OwnerID)
		case "sync_state":
			log.Printf("Client: Received sync_state with %d paths", len(msg.Paths))
			board.ClearRemote("all")
			for _, path := range msg.Paths {
				board.AddRemotePath(path)
			}
		default:
			log.Printf("Client: Unknown message type: %s", msg.Type)
		}
	}
}