#!/bin/bash
cp -a ./oc/. ./tmp
cp ./scripts/functions.sh ./tmp/functions.sh

# inject YAML files to functions.sh
sed -i -e '/namespaceYAMLContent/{r ./res/namespace.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/saYAMLContent/{r ./res/service-account.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/flowAgentYAMLContent/{r ./res/flow-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/packetAgentYAMLContent/{r ./res/packet-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/collectorServiceYAMLContent/{r ./res/collector-service.yml' -e 'd}' ./tmp/functions.sh

# inject updated functions to oc commands
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/oc-netobserv-flows
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/oc-netobserv-packets
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/oc-netobserv-cleanup

rm ./tmp/functions.sh

if [ -z "$1" ]; then
  echo "output generated in tmp folder"
else 
  echo "output generated in $1 folder"
  cp -a ./tmp/. ./"$1"
  rm -rf ./tmp
fi

