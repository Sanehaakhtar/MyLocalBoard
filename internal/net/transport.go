package net

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"github.com/hashicorp/mdns"

	"MyLocalBoard/internal/state"
)

type Transport struct {
	port     int
	asHost   bool
	mdnsSrv  *mdns.Server
	lis      net.Listener
	peers    map[string]net.Conn
	mu       sync.Mutex
	OnRemoteOp func(state.Op)
}

func NewTransport(port int, asHost bool) (*Transport, error) {
	t := &Transport{port: port, asHost: asHost, peers: map[string]net.Conn{}}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil { return nil, err }
	t.lis = l

	// accept loop
	go func() {
		for {
			c, err := t.lis.Accept()
			if err != nil { return }
			t.addPeer(c.RemoteAddr().String(), c)
			go t.handleConn(c)
		}
	}()

	if asHost {
		srv, err := advertise(port)
		if err != nil { log.Println("mdns advertise:", err) }
		t.mdnsSrv = srv
	}
	return t, nil
}

func (t *Transport) DiscoverAndConnect() error {
	return browse(func(addr string) {
		// dial if not self/not connected
		t.mu.Lock()
		_, ok := t.peers[addr]
		t.mu.Unlock()
		if ok { return }
		if c, err := net.Dial("tcp", addr); err == nil {
			t.addPeer(addr, c)
			go t.handleConn(c)
		}
	})
}

func (t *Transport) addPeer(addr string, c net.Conn) {
	t.mu.Lock()
	t.peers[addr] = c
	t.mu.Unlock()
}

func (t *Transport) handleConn(c net.Conn) {
	r := bufio.NewScanner(c)
	for r.Scan() {
		var op state.Op
		if err := json.Unmarshal(r.Bytes(), &op); err == nil && t.OnRemoteOp != nil {
			t.OnRemoteOp(op)
		}
	}
	_ = c.Close()
}

func (t *Transport) Broadcast(op state.Op) {
	msg, _ := json.Marshal(op)
	t.mu.Lock()
	for addr, c := range t.peers {
		if _, err := fmt.Fprintln(c, string(msg)); err != nil {
			_ = c.Close()
			delete(t.peers, addr)
		}
	}
	t.mu.Unlock()
}

func (t *Transport) PeersSummary() string {
	t.mu.Lock(); defer t.mu.Unlock()
	s := "No peers"
	if len(t.peers) > 0 { s = "" }
	for k := range t.peers { s += k + "\n" }
	return s
}

func (t *Transport) SaveSession(path string) error {
	return state.Save(path)
}

func (t *Transport) LoadSession(path string) error {
	return state.Load(path)
}

func (t *Transport) Close() {
	if t.mdnsSrv != nil { t.mdnsSrv.Shutdown() }
	if t.lis != nil { _ = t.lis.Close() }
	t.mu.Lock()
	for _, c := range t.peers { _ = c.Close() }
	t.mu.Unlock()
}

func osHostname() (string, error) { return os.Hostname() }
