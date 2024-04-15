#!/bin/bash
cp -a ./commands/. ./tmp
cp ./scripts/functions.sh ./tmp/functions.sh

if [ -z "$IMAGE" ]; then
  echo "image not provided, keeping current ones"
else 
  echo "updating CLI images to $IMAGE"
  sed -i "/img=/c\img=\"$IMAGE\"" ./tmp/netobserv-flows
  sed -i "/img=/c\img=\"$IMAGE\"" ./tmp/netobserv-packets
fi

if [ -z "$PULL_POLICY" ]; then
  echo "pull policy not provided, keeping current ones"
else 
  echo "updating CLI pull policy to $PULL_POLICY"
  sed -i "/  --image-pull-policy/c\  --image-pull-policy='$PULL_POLICY' \\\\" ./tmp/netobserv-flows
  sed -i "/  --image-pull-policy/c\  --image-pull-policy='$PULL_POLICY' \\\\" ./tmp/netobserv-packets
fi

if [ -z "$K8S_CLI_BIN" ]; then
  echo "ERROR: K8S CLI not provided"
  exit 1
else 
  echo "updating K8S CLI to $K8S_CLI_BIN"
  sed -i "/K8S_CLI_BIN_PATH=/d" ./tmp/functions.sh
  sed -i "/K8S_CLI_BIN=/c\K8S_CLI_BIN=$K8S_CLI_BIN" ./tmp/functions.sh

  mv ./tmp/netobserv-flows ./tmp/"$K8S_CLI_BIN"-netobserv-flows
  mv ./tmp/netobserv-packets ./tmp/"$K8S_CLI_BIN"-netobserv-packets
  mv ./tmp/netobserv-cleanup ./tmp/"$K8S_CLI_BIN"-netobserv-cleanup
fi

# inject YAML files to functions.sh
sed -i -e '/namespaceYAMLContent/{r ./res/namespace.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/saYAMLContent/{r ./res/service-account.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/flowAgentYAMLContent/{r ./res/flow-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/packetAgentYAMLContent/{r ./res/packet-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/collectorServiceYAMLContent/{r ./res/collector-service.yml' -e 'd}' ./tmp/functions.sh

# inject updated functions to commands
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/"$K8S_CLI_BIN"-netobserv-flows
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/"$K8S_CLI_BIN"-netobserv-packets
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/"$K8S_CLI_BIN"-netobserv-cleanup

rm ./tmp/functions.sh

if [ -z "$DIST_DIR" ]; then
  echo "output generated in tmp folder"
else 
  echo "output generated in $DIST_DIR folder"
  cp -a ./tmp/. ./"$DIST_DIR"
  rm -rf ./tmp
fi

