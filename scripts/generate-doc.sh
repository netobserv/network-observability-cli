ADOC=./docs/netobserv_cli.adoc

# Header
echo "// Automatically generated by '$0'. Do not edit, or make the NETOBSERV team aware of the editions." > $ADOC
cat <<EOF >> $ADOC
:_mod-docs-content-type: REFERENCE
[id="network-observability-cli-usage_{context}"]
= Network Observability CLI usage

Users of Netobserv CLI can pass command line arguments to enable features or pass configuration options to the eBPF agent and flowlogs-pipeline.
This document lists all the supported command line options and their possible values.
EOF
echo -e "\n" >> $ADOC

# Flow table
cat <<EOF >> $ADOC
== Flows usage
[cols="1,1,1",options="header"]
|===
| Option | Description | Default
EOF
./build/oc-netobserv flows help >> $ADOC
echo -e "|===\n" >> $ADOC
# Flow example
cat <<EOF >> $ADOC
Example running flow capture with some options:
\`\`\`
oc netobserv flows --enable_pktdrop=true  --enable_rtt=true --enable_filter=true --action=Accept --cidr=0.0.0.0/0 --protocol=TCP --port=49051
\`\`\`
EOF

# Packet table
cat <<EOF >> $ADOC
== Packets usage
[cols="1,1,1",options="header"]
|===
| Option | Description | Default
EOF
./build/oc-netobserv packets help >> $ADOC
echo -e "|===\n" >> $ADOC
# Packet example
cat <<EOF >> $ADOC
Example running packet capture with some options:
\`\`\`
oc netobserv packets --action=Accept --cidr=0.0.0.0/0 --protocol=TCP --port=49051
\`\`\`
EOF

# remove double spaces
sed -i.bak "s/  */ /" $ADOC
# add table rows
sed -i.bak "/^ /s/ --*/|--/" $ADOC
# add table columns
sed -i.bak "/^|/s/(default:/|/" $ADOC
sed -i.bak "/^|/s/: /|/" $ADOC
sed -i.bak "/^|/s/)//" $ADOC

rm ./docs/*.bak