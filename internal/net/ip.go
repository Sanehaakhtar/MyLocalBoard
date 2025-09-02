package net

import (
	"net"
	"log"
)

// GetOutgoingIP gets the preferred outbound IP address
func GetOutgoingIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("Failed to get outgoing IP: %v", err)
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}