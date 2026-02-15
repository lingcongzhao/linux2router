package models

type IPSet struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`        // hash:ip, hash:net, hash:ip,port, list:set
	Family     string   `json:"family"`      // inet, inet6
	HashSize   int      `json:"hash_size"`
	MaxElem    int      `json:"max_elem"`
	NumEntries int      `json:"num_entries"`
	MemSize    int      `json:"mem_size"`
	References int      `json:"references"`  // iptables references
	Timeout    int      `json:"timeout"`     // 0 = no timeout
	Entries    []string `json:"entries"`
}

type IPSetInput struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Family  string `json:"family"`
	Timeout int    `json:"timeout"`
	Comment bool   `json:"comment"`
}

type IPSetEntryInput struct {
	SetName string `json:"set_name"`
	Entry   string `json:"entry"`   // IP, CIDR, IP:port
	Timeout int    `json:"timeout"`
	Comment string `json:"comment"`
}
