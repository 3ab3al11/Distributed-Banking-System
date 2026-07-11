package failover

import (
	"net"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// NodeInfo describes a node in the distributed cluster.
type NodeInfo struct {
	Name string
	URL  string
	Role string
}

// IPToNumeric converts an IPv4 address string to a 32-bit numeric value.
// Returns 0 if the address cannot be parsed.
func IPToNumeric(ipStr string) uint32 {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0
	}
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// ExtractIP extracts the hostname or IP address from a URL string.
// Converts "localhost" to "127.0.0.1" for consistent numeric comparison.
func ExtractIP(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "127.0.0.1"
	}
	host := u.Hostname()
	if host == "" || host == "localhost" {
		return "127.0.0.1"
	}
	return host
}

// nodeNameOrder returns a deterministic priority index for tie-breaking.
// Lower values have higher priority.
func nodeNameOrder(name string) int {
	switch name {
	case "Master":
		return 0
	case "Slave1":
		return 1
	case "Slave2":
		return 2
	case "Slave3":
		return 3
	case "Slave4":
		return 4
	default:
		return 99
	}
}

// CheckHealth returns true if the node at the given URL responds to GET /health
// with an HTTP 200 status code within 2 seconds.
func CheckHealth(nodeURL string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(nodeURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type electionCandidate struct {
	node     NodeInfo
	distance uint32
	order    int
}

// ElectMaster selects the best failover candidate from the given nodes.
//
// The algorithm:
//  1. Exclude the node whose URL matches failedURL (the failed master).
//  2. Check which remaining nodes are online using the isOnline callback.
//  3. Sort online nodes by absolute IP distance to the failed master's IP.
//  4. Break ties deterministically using node name order (Slave1 < Slave2 < Slave3 < Slave4).
//  5. Return the closest online node, or nil if none are available.
func ElectMaster(failedURL string, allNodes []NodeInfo, isOnline func(string) bool) *NodeInfo {
	failedIP := ExtractIP(failedURL)
	failedNumeric := IPToNumeric(failedIP)

	var candidates []electionCandidate

	for _, node := range allNodes {
		// Skip the failed node.
		if node.URL == failedURL {
			continue
		}
		if !isOnline(node.URL) {
			continue
		}

		nodeIP := ExtractIP(node.URL)
		nodeNumeric := IPToNumeric(nodeIP)

		var dist uint32
		if nodeNumeric >= failedNumeric {
			dist = nodeNumeric - failedNumeric
		} else {
			dist = failedNumeric - nodeNumeric
		}

		candidates = append(candidates, electionCandidate{
			node:     node,
			distance: dist,
			order:    nodeNameOrder(node.Name),
		})
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].order < candidates[j].order
	})

	return &candidates[0].node
}
