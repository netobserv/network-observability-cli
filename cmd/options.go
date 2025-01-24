package cmd

type option struct {
	all     []optionItem
	current int
}

type optionItem struct {
	name string
	ids  []string
}

var (
	allOptions = "All"
	noOptions  = "None"

	// displays
	rawDisplay           = "Raw"
	standardDisplay      = "Standard"
	pktDropFeature       = "pktDrop"
	dnsFeature           = "dnsTracking"
	rttFeature           = "flowRTT"
	networkEventsDisplay = "networkEvents"
	pktTranslation       = "packetTranslation"
	udnMapping           = "udnMapping"
	display              = option{
		all: []optionItem{
			// exclusive displays
			{name: rawDisplay},
			{name: standardDisplay},
			// per feature displays
			{name: "Packet drops", ids: []string{pktDropFeature}},
			{name: "DNS", ids: []string{dnsFeature}},
			{name: "RTT", ids: []string{rttFeature}},
			{name: "Network events", ids: []string{networkEventsDisplay}},
			{name: "Packet translation", ids: []string{pktTranslation}},
			{name: "UDN mapping", ids: []string{udnMapping}},
			// all features display
			{name: allOptions, ids: []string{pktDropFeature, dnsFeature, rttFeature, networkEventsDisplay, pktTranslation, udnMapping}},
		},
		// standard display by default
		current: 1,
	}

	// enrichments
	enrichment = option{
		all: []optionItem{
			// no enrichment
			{name: noOptions},
			// per field enrichments
			{name: "Cluster", ids: []string{"ClusterName"}},
			{name: "Zone", ids: []string{"SrcZone", "DstZone"}},
			{name: "Host", ids: []string{"SrcK8S_HostIP", "DstK8S_HostIP", "SrcK8S_HostName", "DstK8S_HostName", "FlowDirection"}},
			{name: "Namespace", ids: []string{"SrcK8S_Namespace", "DstK8S_Namespace"}},
			{name: "Owner", ids: []string{"SrcK8S_OwnerType", "DstK8S_OwnerType", "SrcK8S_OwnerName", "DstK8S_OwnerName", "SrcK8S_Namespace", "DstK8S_Namespace"}},
			{name: "Resource", ids: []string{"SrcK8S_Type", "DstK8S_Type", "SrcK8S_Name", "DstK8S_Name", "SrcK8S_Namespace", "DstK8S_Namespace"}},
			{name: "SubnetLabel", ids: []string{"SrcSubnetLabel", "DstSubnetLabel"}},
			// all fields
			{name: allOptions, ids: []string{
				"ClusterName",
				"SrcZone", "DstZone",
				"SrcK8S_HostIP", "DstK8S_HostIP", "SrcK8S_HostName", "DstK8S_HostName",
				"SrcK8S_Namespace", "DstK8S_Namespace",
				"SrcK8S_OwnerType", "DstK8S_OwnerType", "SrcK8S_OwnerName", "DstK8S_OwnerName",
				"SrcK8S_Type", "DstK8S_Type", "SrcK8S_Name", "DstK8S_Name",
				"SrcSubnetLabel", "DstSubnetLabel",
			}},
		},
		// resource enrichment by default
		current: 6,
	}
)

func (opt *option) getCurrentItem() optionItem {
	return opt.all[opt.current]
}

func (opt *option) prev() {
	opt.current += len(opt.all) - 1
	opt.current %= len(opt.all)
}

func (opt *option) next() {
	opt.current++
	opt.current %= len(opt.all)
}
