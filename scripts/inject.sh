#!/bin/bash
cp -a ./commands/. ./tmp
cp ./scripts/functions.sh ./tmp/functions.sh

if [ -z "$IMAGE" ]; then
  echo "image not provided, keeping current one"
else 
  echo "updating CLI images to $IMAGE"
  sed -i.bak -e "/img=/c\img=\"$IMAGE\"" ./tmp/netobserv
fi

if [ -z "$PULL_POLICY" ]; then
  echo "pull policy not provided, keeping current one"
else 
  echo "updating CLI pull policy to $PULL_POLICY"
  sed -i.bak -e "/  --i.bakmage-pull-policy/c\  --i.bakmage-pull-policy='$PULL_POLICY' \\\\" ./tmp/netobserv
fi

if [ -z "$VERSION" ]; then
  echo "version not provided, keeping current one"
else 
  echo "updating CLI version to $VERSION"
  sed -i.bak -e "/version=/c\version=\"$VERSION\"" ./tmp/netobserv
fi

prefix=
if [ -z "$KREW_PLUGIN" ] || [ "$KREW_PLUGIN" = "false" ]; then
  if [ -z "$K8S_CLI_BIN" ]; then
    echo "ERROR: K8S CLI not provided"
    exit 1
  fi 
  echo "updating K8S CLI to $K8S_CLI_BIN"
  # remove unecessary call
  sed -i.bak -e "/K8S_CLI_BIN_PATH=/d" ./tmp/functions.sh
  # replace only first match to force default
  sed -i.bak -e "0,/K8S_CLI_BIN=/c\K8S_CLI_BIN=$K8S_CLI_BIN" ./tmp/functions.sh
  # prefix with oc / kubectl for local install
  prefix="$K8S_CLI_BIN-"
  echo "prefixing with $prefix"
  mv ./tmp/netobserv ./tmp/"$prefix"netobserv
fi

# inject YAML files to functions.sh
sed -i.bak -e '/namespaceYAMLContent/{r ./res/namespace.yml' -e 'd}' ./tmp/functions.sh
sed -i.bak -e '/saYAMLContent/{r ./res/service-account.yml' -e 'd}' ./tmp/functions.sh
sed -i.bak -e '/flowAgentYAMLContent/{r ./res/flow-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i.bak -e '/packetAgentYAMLContent/{r ./res/packet-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i.bak -e '/collectorServiceYAMLContent/{r ./res/collector-service.yml' -e 'd}' ./tmp/functions.sh

# inject updated functions to commands
sed -i.bak -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/"$prefix"netobserv

if [ -z "$3" ]; then
  echo "pull policy not provided, keeping current ones"
else 
  echo "updating CLI pull policy to $3"
  sed -i.bak -e "/  --i.bakmage-pull-policy/c\  --i.bakmage-pull-policy='$3' \\\\" ./tmp/oc-netobserv-flows
  sed -i.bak -e "/  --i.bakmage-pull-policy/c\  --i.bakmage-pull-policy='$3' \\\\" ./tmp/oc-netobserv-packets
fi

rm ./tmp/functions.sh
rm ./tmp/*.bak

if [ -z "$DIST_DIR" ]; then
  echo "output generated in tmp folder"
else 
  echo "output generated in $DIST_DIR folder"
  cp -a ./tmp/. ./"$DIST_DIR"
  rm -rf ./tmp
fi

