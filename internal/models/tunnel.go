package models

// GRETunnel represents a GRE tunnel interface
type GRETunnel struct {
	Name      string   `json:"name"`
	Mode      string   `json:"mode"` // gre, gretap, ip6gre
	Local     string   `json:"local"`
	Remote    string   `json:"remote"`
	Key       string   `json:"key,omitempty"`
	TTL       int      `json:"ttl,omitempty"`
	State     string   `json:"state"` // UP, DOWN
	MTU       int      `json:"mtu,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
}

// GRETunnelInput represents input for creating a GRE tunnel
type GRETunnelInput struct {
	Name    string `json:"name"`
	Local   string `json:"local"`
	Remote  string `json:"remote"`
	Key     string `json:"key,omitempty"`
	TTL     int    `json:"ttl,omitempty"`
	Address string `json:"address,omitempty"` // IP address to assign (CIDR format)
}

// VXLANTunnel represents a VXLAN tunnel interface
type VXLANTunnel struct {
	Name      string   `json:"name"`
	VNI       int      `json:"vni"` // VXLAN Network Identifier
	Local     string   `json:"local,omitempty"`
	Remote    string   `json:"remote,omitempty"` // unicast remote
	Group     string   `json:"group,omitempty"`  // multicast group
	DstPort   int      `json:"dstport"`          // destination port (default 4789)
	Dev       string   `json:"dev,omitempty"`    // underlying device
	State     string   `json:"state"`            // UP, DOWN
	MTU       int      `json:"mtu,omitempty"`
	MAC       string   `json:"mac,omitempty"` // MAC address
	Addresses []string `json:"addresses,omitempty"`
}

// VXLANTunnelInput represents input for creating a VXLAN tunnel
type VXLANTunnelInput struct {
	Name    string `json:"name"`
	VNI     int    `json:"vni"`
	Local   string `json:"local,omitempty"`
	Remote  string `json:"remote,omitempty"`
	Group   string `json:"group,omitempty"`
	DstPort int    `json:"dstport,omitempty"`
	Dev     string `json:"dev,omitempty"`
	MAC     string `json:"mac,omitempty"`     // optional custom MAC, auto-generated if empty
	Address string `json:"address,omitempty"` // IP address to assign (CIDR format)
}

// WireGuardTunnel represents a WireGuard tunnel interface
type WireGuardTunnel struct {
	Name       string          `json:"name"`
	PublicKey  string          `json:"public_key"`
	ListenPort int             `json:"listen_port,omitempty"`
	FwMark     string          `json:"fwmark,omitempty"`
	State      string          `json:"state"` // UP, DOWN
	Peers      []WireGuardPeer `json:"peers,omitempty"`
	Addresses  []string        `json:"addresses,omitempty"`
}

// WireGuardPeer represents a WireGuard peer
type WireGuardPeer struct {
	PublicKey           string   `json:"public_key"`
	PresharedKey        string   `json:"preshared_key,omitempty"`
	Endpoint            string   `json:"endpoint,omitempty"`
	AllowedIPs          []string `json:"allowed_ips"`
	LatestHandshake     string   `json:"latest_handshake,omitempty"`
	TransferRx          uint64   `json:"transfer_rx"`
	TransferTx          uint64   `json:"transfer_tx"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty"`
}

// WireGuardTunnelInput represents input for creating a WireGuard tunnel
type WireGuardTunnelInput struct {
	Name       string `json:"name"`
	PrivateKey string `json:"private_key,omitempty"` // if empty, generate new
	ListenPort int    `json:"listen_port,omitempty"`
	Address    string `json:"address,omitempty"` // IP address to assign
}

// WireGuardInterfaceInput represents input for updating a WireGuard interface
type WireGuardInterfaceInput struct {
	ListenPort int    `json:"listen_port,omitempty"`
	Address    string `json:"address,omitempty"`
}

// WireGuardPeerInput represents input for adding a WireGuard peer
type WireGuardPeerInput struct {
	Interface           string `json:"interface"`
	PublicKey           string `json:"public_key"`
	PresharedKey        string `json:"preshared_key,omitempty"`
	Endpoint            string `json:"endpoint,omitempty"`
	AllowedIPs          string `json:"allowed_ips"` // comma-separated
	PersistentKeepalive int    `json:"persistent_keepalive,omitempty"`
}
