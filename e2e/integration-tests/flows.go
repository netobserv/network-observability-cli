package integration_tests

type Flowlog struct {
	// Source
	SrcPort          int
	SrcK8S_Type      string
	SrcK8S_Name      string
	SrcK8S_HostName  string
	SrcK8S_OwnerType string
	SrcAddr          string
	SrcMac           string
	SrcK8S_Namespace string
	// Destination
	DstPort          int
	DstK8S_Type      string
	DstK8S_Name      string
	DstK8S_HostName  string
	DstK8S_OwnerType string
	DstAddr          string
	DstMac           string
	DstK8S_HostIP    string
	DstK8S_Namespace string
	// Protocol
	Proto    int
	IcmpCode int
	IcmpType int
	Dscp     int
	// TODO: check if Flags supposed to be []string?
	Flags int
	// Time
	TimeReceived    int
	TimeFlowEndMs   int
	TimeFlowStartMs int
	// Interface
	IfDirection  int
	IfDirections []int
	Interfaces   []string
	Etype        int
	// Others
	Packets         int
	Bytes           int
	Duplicate       bool
	AgentIP         string
	Sampling        int
	HashId          string `json:"_HashId,omitempty"`
	IsFirst         bool   `json:"_IsFirst,omitempty"`
	RecordType      string `json:"_RecordType,omitempty"`
	NumFlowLogs     int    `json:"numFlowLogs,omitempty"`
	K8S_ClusterName string `json:"K8S_ClusterName,omitempty"`
	// Zone
	SrcK8S_Zone string `json:"SrcK8S_Zone,omitempty"`
	DstK8S_Zone string `json:"DstK8S_Zone,omitempty"`
	// DNS
	DnsLatencyMs         int    `json:"DnsLatencyMs,omitempty"`
	DnsFlagsResponseCode string `json:"DnsFlagsResponseCode,omitempty"`
	// Packet Drop
	PktDropBytes           int    `json:"PktDropBytes,omitempty"`
	PktDropPackets         int    `json:"PktDropPackets,omitempty"`
	PktDropLatestState     string `json:"PktDropLatestState,omitempty"`
	PktDropLatestDropCause string `json:"PktDropLatestDropCause,omitempty"`
	// RTT
	TimeFlowRttNs int `json:"TimeFlowRttNs,omitempty"`
	// Packet Translation
	XlatDstAddr          string `json:"XlatDstAddr,omitempty"`
	XlatDstK8S_Name      string `json:"XlatDstK8S_Name,omitempty"`
	XlatDstK8S_Namespace string `json:"XlatDstK8S_Namespace,omitempty"`
	XlatDstK8S_Type      string `json:"XlatDstK8S_Type,omitempty"`
	XlatDstPort          int    `json:"XlatDstPort,omitempty"`
	XlatSrcAddr          string `json:"XlatSrcAddr,omitempty"`
	XlatSrcK8S_Name      string `json:"XlatSrcK8S_Name,omitempty"`
	XlatSrcK8S_Namespace string `json:"XlatSrcK8S_Namespace,omitempty"`
	ZoneId               int    `json:"ZoneId,omitempty"`
	// Network Events
	NetworkEvents []NetworkEvent `json:"NetworkEvents,omitempty"`
	// Secondary Network
	SrcK8S_NetworkName string `json:"SrcK8S_NetworkName,omitempty"`
	DstK8S_NetworkName string `json:"DstK8S_NetworkName,omitempty"`
	// UDN
	Udns []string `json:"Udns,omitempty"`
}
type NetworkEvent struct {
	Action    string `json:"Action,omitempty"`
	Type      string `json:"Type,omitempty"`
	Name      string `json:"Name,omitempty"`
	Namespace string `json:"Namespace,omitempty"`
	Direction string `json:"Direction,omitempty"`
	Feature   string `json:"Feature,omitempty"`
}
