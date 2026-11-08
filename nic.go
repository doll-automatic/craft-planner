package main

import "net"

// externalInterfaceIndexes は、外部向けに選ばれる IPv4 を持つ NIC の番号。
// UDP は接続を張らないので、宛先へ実際には送らずに OS の経路選択だけを使える。
// 外部への経路が無い NIC（APIPA だけを持つ NIC など）は選ばれない。
func externalInterfaceIndexes() map[int]bool {
	conn, err := net.Dial("udp", "192.0.2.1:80")
	if err != nil {
		return nil
	}
	defer conn.Close()
	ua, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || !routableIPv4(ua.IP.To4()) {
		return nil
	}
	want := ua.IP.String()
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	indexes := map[int]bool{}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipv4Of(addr)
			if ip != nil && ip.String() == want {
				indexes[iface.Index] = true
			}
		}
	}
	return indexes
}
