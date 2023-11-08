package peer_service

import (
	"github.com/gofiber/fiber/v2/log"
	"net"
)

func findIp() string {
	var resultIp net.IP

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Infof("Enum interfaces failed", err)
		return ""
	}
loop:
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagLoopback != 0) || (iface.Flags&net.FlagUp == 0) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			var ipv4 net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				ipv4 = ip.To4()
			case *net.IPAddr:
				ip = v.IP
				ipv4 = ip.To4()
			}
			if resultIp == nil {
				resultIp = ip
				if ipv4 != nil {
					break loop
				}
			} else if ipv4 != nil {
				resultIp = ip
				break loop
			}
		}
	}
	if resultIp == nil {
		return ""
	}
	return resultIp.String()
}
