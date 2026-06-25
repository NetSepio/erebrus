package store

import (
	"context"
	"database/sql"
	"errors"
	"net"
)

// ErrSubnetExhausted is returned when no free address remains.
var ErrSubnetExhausted = errors.New("wireguard subnet exhausted")

// txAllocateIP finds the lowest free host address in subnet (CIDR with the
// server address as host bits, e.g. "10.0.0.1/16") not already assigned to a
// peer, and returns it as a /32 CIDR. Runs inside the caller's transaction so
// the read-then-insert is atomic against concurrent provisioning.
//
// The subnet's own host address (the server, e.g. 10.0.0.1), the network
// address, and the broadcast address are reserved.
func txAllocateIP(ctx context.Context, tx *sql.Tx, subnet string) (string, error) {
	serverIP, ipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return "", err
	}

	reserved := map[string]struct{}{}
	// network + broadcast + server host
	reserved[ipnet.IP.String()] = struct{}{}
	reserved[broadcastAddr(ipnet).String()] = struct{}{}
	reserved[serverIP.String()] = struct{}{}

	rows, err := tx.QueryContext(ctx, `SELECT wg_allowed_ip FROM peers`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			return "", err
		}
		if ip, _, err := net.ParseCIDR(cidr); err == nil {
			reserved[ip.String()] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	for ip := cloneIP(ipnet.IP.Mask(ipnet.Mask)); ipnet.Contains(ip); incIP(ip) {
		addr := ip.String()
		if _, taken := reserved[addr]; taken {
			continue
		}
		return addr + "/32", nil
	}
	return "", ErrSubnetExhausted
}

func cloneIP(ip net.IP) net.IP {
	out := make(net.IP, len(ip))
	copy(out, ip)
	return out
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func broadcastAddr(n *net.IPNet) net.IP {
	var b net.IP
	if len(n.IP) == 4 {
		b = net.ParseIP("0.0.0.0").To4()
	} else {
		b = net.ParseIP("::")
	}
	for i := 0; i < len(n.IP); i++ {
		b[i] = n.IP[i] | ^n.Mask[i]
	}
	return b
}
