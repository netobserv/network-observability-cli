#!/bin/bash
set -x

cp -a ./commands/. ./tmp
cp ./scripts/functions.sh ./tmp/functions.sh
cp ./scripts/dependencies_check.sh ./tmp/dependencies_check.sh

if [ -z "$IMAGE" ]; then
  echo "image not provided, keeping current one"
else
  echo "updating CLI images to $IMAGE"
  sed -i.bak "s|^img=.*|img=\"$IMAGE\"|" ./tmp/netobserv
fi

if [ -z "$PULL_POLICY" ]; then
  echo "pull policy not provided, keeping current one"
else
  echo "updating CLI pull policy to $PULL_POLICY"
  sed -i.bak "s/--image-pull-policy=.*/--image-pull-policy='$PULL_POLICY' \\\\/" ./tmp/netobserv
fi

if [ -z "$AGENT_IMAGE" ]; then
  echo "eBPF agent image not provided, keeping current one"
else
  echo "updating eBPF agent images to $AGENT_IMAGE"
  sed -i.bak "s|^agentImg=.*|agentImg=\"$AGENT_IMAGE\"|" ./tmp/functions.sh
fi

if [ -z "$VERSION" ]; then
  echo "version not provided, keeping current one"
else
  echo "updating CLI version to $VERSION"
  sed -i.bak "s/^version=.*/version=\"$VERSION\"/" ./tmp/netobserv
fi

if [ -z "$REQUIRED_YQ_VERSION" ]; then
  echo "require yq version is not set, keeping the current version"
else
  echo "updating dependencies_check to check for $REQUIRED_YQ_VERSION"
  sed -i.bak "s/^required_yq_version=.*/required_yq_version=\"$REQUIRED_YQ_VERSION\"/" ./tmp/netobserv
fi

if [ -z "$SUPPORTED_ARCHS" ]; then
  echo "list of supported archs is not set"
else
  echo "updating dependencies_check with $SUPPORTED_ARCHS values"
  sed -i.bak "s/^supported_archs=.*/supported_archs=\"$SUPPORTED_ARCHS\"/" ./tmp/netobserv
fi

prefix=
if [ -z "$KREW_PLUGIN" ] || [ "$KREW_PLUGIN" = "false" ]; then
  if [ -z "$K8S_CLI_BIN" ]; then
    echo "ERROR: K8S CLI not provided"
    exit 1
  fi
  echo "updating K8S CLI to $K8S_CLI_BIN"
  # remove beginning lines
  sed -i.bak '1,/K8S_CLI_BIN_PATH=/d' ./tmp/functions.sh
  # replace only first match to force default
  sed -i.bak "s/^K8S_CLI_BIN=.*/K8S_CLI_BIN=$K8S_CLI_BIN/" ./tmp/functions.sh
  # prefix with oc / kubectl for local install
  prefix="$K8S_CLI_BIN-"
  echo "prefixing with $prefix"
  mv ./tmp/netobserv ./tmp/"$prefix"netobserv
fi

# inject YAML files to functions.sh
sed -i.bak '/namespaceYAMLContent/{r ./res/namespace.yml
d
}' ./tmp/functions.sh
sed -i.bak '/saYAMLContent/{r ./res/service-account.yml
d
}' ./tmp/functions.sh
sed -i.bak '/flowAgentYAMLContent/{r ./res/flow-capture.yml
d
}' ./tmp/functions.sh
sed -i.bak '/packetAgentYAMLContent/{r ./res/packet-capture.yml
d
}' ./tmp/functions.sh
sed -i.bak '/collectorServiceYAMLContent/{r ./res/collector-service.yml
d
}' ./tmp/functions.sh

# inject updated functions to commands
sed -i.bak '/^source "\.\/scripts\/functions\.sh"/{r ./tmp/functions.sh
d
}' ./tmp/"$prefix"netobserv

# inject updated dependencies_check to commands
sed -i.bak '/^source "\.\/scripts\/dependencies_check\.sh"/{r ./tmp/dependencies_check.sh
d
}' ./tmp/"$prefix"netobserv

if [ -z "$3" ]; then
  echo "pull policy not provided, keeping current ones"
else
  echo "updating CLI pull policy to $3"
  sed -i.bak "s/--image-pull-policy=.*/--image-pull-policy='$3' \\\\/" ./tmp/oc-netobserv-flows
  sed -i.bak "s/--image-pull-policy=.*/--image-pull-policy='$3' \\\\/" ./tmp/oc-netobserv-packets
fi

rm ./tmp/functions.sh
rm ./tmp/dependencies_check.sh
rm ./tmp/*.bak

if [ -z "$DIST_DIR" ]; then
  echo "output generated in tmp folder"
else
  echo "output generated in $DIST_DIR folder"
  cp -a ./tmp/. ./"$DIST_DIR"
  rm -rf ./tmp
fi

