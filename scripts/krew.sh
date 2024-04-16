#!/bin/bash

SHA=$(sha256sum netobserv-cli.tar.gz | awk '{print $1}')

URL="<URL>"
if [[ $VERSION = +([[:digit:]]).+([[:digit:]]).+([[:digit:]]) ]]; then
  URL="https://github.com/netobserv/network-observability-cli/releases/download/v${VERSION}/netobserv-cli.tar.gz"
fi


indexYaml='
apiVersion: krew.googlecontainertools.github.com/v1alpha2
kind: Plugin
metadata:
  name: netobserv
spec:
  version: "'${VERSION}'"
  homepage: https://github.com/netobserv/network-observability-cli
  shortDescription: "Lightweight Flow and Packet visualization tool"
  description: |
    Deploys NetObserv eBPF agent on your k8s cluster to collect flows 
    or packets from nodes network interfaces and streams data to a local 
    collector for analysis and visualization. 
  platforms:
  - selector:
      matchExpressions:
      - key: "os"
        operator: "In"
        values:
        - darwin
        - linux
    uri: "'${URL}'"
    sha256: "'${SHA}'"
    files:
    - from: "build/netobserv"
      to: "netobserv"
    - from: "LICENSE"
      to: "."
    bin: netobserv
'

echo "Copy the following YAML and submit it to https://github.com/kubernetes-sigs/krew-index for release:" 
echo "${indexYaml}"

# github todo release notes
# check .github/workflows/release.yml for usage
mkdir -p ./tmp
echo "TODO: 
- Submit updated index to https://github.com/kubernetes-sigs/krew-index to update plugin:
\`\`\`yaml${indexYaml}\`\`\`
- Click on 'generate release notes' above and publish" > ./tmp/release.md
