#!/usr/bin/env bash
set -eux

# get either oc (favorite) or kubectl paths
K8S_CLI_BIN_PATH=$( which oc 2>/dev/null || which kubectl 2>/dev/null )
K8S_CLI_BIN=$( basename "${K8S_CLI_BIN_PATH}" )

DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && cd ../ && pwd )

KIND_CLUSTER_NAME="netobserv-cli-cluster"
KIND_IMAGE="kindest/node:v1.31.0"

# deploy_kind installs the kind cluster
deploy_kind() {
  cat <<EOF | kind create cluster --image ${KIND_IMAGE} --config=- --kubeconfig="${DIR}"/kubeconfig --name ${KIND_CLUSTER_NAME}
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
    podSubnet: $NET_CIDR_IPV4,$NET_CIDR_IPV6
    serviceSubnet: $SVC_CIDR_IPV4,$SVC_CIDR_IPV6
    ipFamily: $IP_FAMILY
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
        extraArgs:
            v: "5"
    controllerManager:
        extraArgs:
            v: "5"
    scheduler:
        extraArgs:
            v: "5"
- role: worker
- role: worker
EOF
}

# print_success prints a little success message at the end of the script
print_success() {
  set +x
  echo "Your kind cluster was created successfully"
  echo "Run the following to load the kubeconfig:"
  echo "export KUBECONFIG=${DIR}/kubeconfig"
  set -x
}

IP_FAMILY=${IP_FAMILY:-dual}
NET_CIDR_IPV4=${NET_CIDR_IPV4:-10.244.0.0/16}
SVC_CIDR_IPV4=${SVC_CIDR_IPV4:-10.96.0.0/16}
NET_CIDR_IPV6=${NET_CIDR_IPV6:-fd00:10:244::/48}
SVC_CIDR_IPV6=${SVC_CIDR_IPV6:-fd00:10:96::/112}

# At the minimum, deploy the kind cluster
deploy_kind
export KUBECONFIG=${DIR}/kubeconfig
${K8S_CLI_BIN} label node ${KIND_CLUSTER_NAME}-worker node-role.kubernetes.io/worker=
${K8S_CLI_BIN} label node ${KIND_CLUSTER_NAME}-worker2 node-role.kubernetes.io/worker=

# Print success at the end of this script
print_success
