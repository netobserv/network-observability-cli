package integrationtests

type Flowlog struct {
	// Source
	SrcPort         int
	SrcK8sType      string `json:"SrcK8S_Type"`
	SrcK8sName      string `json:"SrcK8S_Name"`
	SrcK8sHostName  string `json:"SrcK8S_HostName"`
	SrcK8sOwnerType string `json:"SrcK8S_OwnerType"`
	SrcAddr         string
	SrcMac          string
	SrcK8sNamespace string `json:"SrcK8S_Namespace"`
	// Destination
	DstPort         int
	DstK8sType      string `json:"DstK8S_Type"`
	DstK8sName      string `json:"DstK8S_Name"`
	DstK8sHostName  string `json:"DstK8S_HostName"`
	DstK8sOwnerType string `json:"DstK8S_OwnerType"`
	DstAddr         string
	DstMac          string
	DstK8sHostIP    string `json:"DstK8S_HostIP,omitempty"`
	DstK8sNamespace string `json:"DstK8S_Namespace"`
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
	Packets        int
	Bytes          int
	Duplicate      bool
	AgentIP        string
	Sampling       int
	HashID         string `json:"_HashId,omitempty"`
	IsFirst        bool   `json:"_IsFirst,omitempty"`
	RecordType     string `json:"_RecordType,omitempty"`
	NumFlowLogs    int    `json:"numFlowLogs,omitempty"`
	K8SClusterName string `json:"K8S_ClusterName,omitempty"`
	// Zone
	SrcK8SZone string `json:"SrcK8S_Zone,omitempty"`
	DstK8SZone string `json:"DstK8S_Zone,omitempty"`
	// DNS
	DNSLatencyMs         int    `json:"DnsLatencyMs,omitempty"`
	DNSFlagsResponseCode string `json:"DnsFlagsResponseCode,omitempty"`
	// Packet Drop
	PktDropBytes           int    `json:"PktDropBytes,omitempty"`
	PktDropPackets         int    `json:"PktDropPackets,omitempty"`
	PktDropLatestState     string `json:"PktDropLatestState,omitempty"`
	PktDropLatestDropCause string `json:"PktDropLatestDropCause,omitempty"`
	// RTT
	TimeFlowRttNs int `json:"TimeFlowRttNs,omitempty"`
	// Packet Translation
	XlatDstAddr         string `json:"XlatDstAddr,omitempty"`
	XlatDstK8sName      string `json:"XlatDstK8S_Name,omitempty"`
	XlatDstK8sNamespace string `json:"XlatDstK8S_Namespace,omitempty"`
	XlatDstK8sType      string `json:"XlatDstK8S_Type,omitempty"`
	XlatDstPort         int    `json:"XlatDstPort,omitempty"`
	XlatSrcAddr         string `json:"XlatSrcAddr,omitempty"`
	XlatSrcK8sName      string `json:"XlatSrcK8S_Name,omitempty"`
	XlatSrcK8sNamespace string `json:"XlatSrcK8S_Namespace,omitempty"`
	ZoneID              int    `json:"ZoneId,omitempty"`
	// Network Events
	NetworkEvents []NetworkEvent `json:"NetworkEvents,omitempty"`
	// Secondary Network
	SrcK8sNetworkName string `json:"SrcK8S_NetworkName,omitempty"`
	DstK8sNetworkName string `json:"DstK8S_NetworkName,omitempty"`
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
