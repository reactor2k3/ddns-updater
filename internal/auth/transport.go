package auth

import "net"

// privateAddress deliberately accepts IP literals, without trusting DNS or proxy headers.
func privateAddress(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback())
}

func privatePeer(address string) bool {
	host, _, err := net.SplitHostPort(address)
	return err == nil && privateAddress(host)
}
