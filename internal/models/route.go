package models

type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
	Scope       string `json:"scope"`
	Protocol    string `json:"protocol"`
	Type        string `json:"type"`
	Table       string `json:"table"`
	Source      string `json:"source"`
	Flags       string `json:"flags"`
}

type RouteInput struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
	Table       string `json:"table"`
}

type RoutingTable struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
