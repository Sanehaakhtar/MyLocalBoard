package net

import (
	"log"
	"net"
)

// GetOutgoingIP finds the preferred local IP address for the host to share.
func GetOutgoingIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// If we can't connect to the internet, fall back to checking local interfaces.
		return getLocalIPFallback()
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// getLocalIPFallback is used on networks without internet access.
func getLocalIPFallback() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// Check the address type and if it is not a loopback, return it.
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	log.Println("No suitable local IP found, link generation may fail.")
	return "127.0.0.1", nil // Fallback to loopback
}