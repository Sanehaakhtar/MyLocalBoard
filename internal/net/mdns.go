package net // Or whatever package name you prefer, like 'networking'

import (
	"fmt"
	"net"
	"os"

	"github.com/hashicorp/mdns"
)

const serviceType = "_localboard._tcp"

// advertise service for hosts.
// THIS IS THE CORRECTED, MODERN VERSION.
func advertise(port int) (*mdns.Server, error) {
	// Get host and IP. Your existing helper functions are great for this.
	host, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("could not get hostname: %w", err)
	}

	info := []string{"LocalBoard"}

	// 1. Create the service object using the simple helper function.
	// This automatically creates all the necessary DNS records (SRV, TXT, A, etc.) for you.
	service, err := mdns.NewMDNSService(
		host,        // Instance name of the service (e.g., "My-Laptop")
		serviceType, // Type of service (_localboard._tcp)
		"",          // Domain (leave empty for ".local")
		"",          // Hostname (leave empty to use the OS hostname)
		port,        // The port your service is running on
		nil,         // IPs (leave nil to auto-detect IPs)
		info,        // Your TXT record info
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// 2. Create a new mDNS server and pass it the service object in a Config.
	// This is much simpler than manually building the Zone and Records.
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("failed to start mDNS server: %w", err)
	}

	return server, nil
}

// browse function is already correct and modern. No changes needed.
func browse(found func(addr string)) error {
	entries := make(chan *mdns.ServiceEntry, 8)
	go func() {
		for e := range entries {
			if e.AddrV4 == nil || e.Port == 0 {
				continue
			}
			found(fmt.Sprintf("%s:%d", e.AddrV4.String(), e.Port))
		}
	}()
	return mdns.Lookup(serviceType, entries)
}

// firstIPv4 function is also correct and robust. No changes needed.
func firstIPv4() net.IP {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		// Ignore loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return ipnet.IP.To4()
			}
		}
	}
	return net.IPv4(127, 0, 0, 1)
}