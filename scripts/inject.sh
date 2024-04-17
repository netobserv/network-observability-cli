#!/bin/bash
cp -a ./commands/. ./tmp
cp ./scripts/functions.sh ./tmp/functions.sh

if [ -z "$IMAGE" ]; then
  echo "image not provided, keeping current one"
else 
  echo "updating CLI images to $IMAGE"
  sed -i "/img=/c\img=\"$IMAGE\"" ./tmp/netobserv
fi

if [ -z "$PULL_POLICY" ]; then
  echo "pull policy not provided, keeping current one"
else 
  echo "updating CLI pull policy to $PULL_POLICY"
  sed -i "/  --image-pull-policy/c\  --image-pull-policy='$PULL_POLICY' \\\\" ./tmp/netobserv
fi

if [ -z "$VERSION" ]; then
  echo "version not provided, keeping current one"
else 
  echo "updating CLI version to $VERSION"
  sed -i "/version=/c\version=\"$VERSION\"" ./tmp/netobserv
fi

prefix=
if [ -z "$KREW_PLUGIN" ] || [ "$KREW_PLUGIN" = "false" ]; then
  if [ -z "$K8S_CLI_BIN" ]; then
    echo "ERROR: K8S CLI not provided"
    exit 1
  fi 
  echo "updating K8S CLI to $K8S_CLI_BIN"
  # remove unecessary call
  sed -i "/K8S_CLI_BIN_PATH=/d" ./tmp/functions.sh
  # replace only first match to force default
  sed -i "0,/K8S_CLI_BIN=/c\K8S_CLI_BIN=$K8S_CLI_BIN" ./tmp/functions.sh
  # prefix with oc / kubectl for local install
  prefix="$K8S_CLI_BIN-"
  echo "prefixing with $prefix"
  mv ./tmp/netobserv ./tmp/"$prefix"netobserv
fi

# inject YAML files to functions.sh
sed -i -e '/namespaceYAMLContent/{r ./res/namespace.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/saYAMLContent/{r ./res/service-account.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/flowAgentYAMLContent/{r ./res/flow-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/packetAgentYAMLContent/{r ./res/packet-capture.yml' -e 'd}' ./tmp/functions.sh
sed -i -e '/collectorServiceYAMLContent/{r ./res/collector-service.yml' -e 'd}' ./tmp/functions.sh

# inject updated functions to commands
sed -i -e '/source.*/{r ./tmp/functions.sh' -e 'd}' ./tmp/"$prefix"netobserv

rm ./tmp/functions.sh

if [ -z "$DIST_DIR" ]; then
  echo "output generated in tmp folder"
else 
  echo "output generated in $DIST_DIR folder"
  cp -a ./tmp/. ./"$DIST_DIR"
  rm -rf ./tmp
fi

