#!/usr/bin/env bash

set +u

function check_dependencies() {
  LOCAL_YQ="/tmp/yq"
  if [ -x "$LOCAL_YQ" ]; then
    YQ_BIN=$LOCAL_YQ
  else
    # Check if yq is installed
    YQ_BIN=$(which yq 2>/dev/null)
  fi
  if [ -z "$YQ_BIN" ]; then
    echo "Error: 'yq' is not installed or not in PATH."
    install_yq "$2"
    return
  fi

  # Get the current version of yq
  current_yq_version=$("$YQ_BIN" --version | awk '{print $NF}')
  required_yq_version="$1"
  # Compare versions
  compare_versions "${current_yq_version#v}" "${required_yq_version#v}"

  if [ "$result" -eq 0 ]; then
    echo "Installing yq version $required_yq_version. Found version $current_yq_version."
    install_yq "$2"
  else
    echo "'yq' is already up to date (version $current_yq_version)."
  fi
}

function compare_versions() {
    IFS="." read -r -a ver1 <<< "$1"
    IFS="." read -r -a ver2 <<< "$2"

    for ((i = 0; i < ${#ver1[@]} || i < ${#ver2[@]}; i++)); do
        v1=${ver1[i]:-0} # Default to 0 if unset
        v2=${ver2[i]:-0}
        if ((v1 < v2)); then
            result=0 # less than
            return
        elif ((v1 > v2)); then
            result=1 # greater than
            return
        fi
    done
    result=2 # equal
}

function install_yq() {
  OS=$(uname | tr '[:upper:]' '[:lower:]') # Get the OS type (linux or darwin)
  supported_archs="$1"
  YQ_BIN="$LOCAL_YQ"
  for arch in $supported_archs; do
    echo "Attempting to download yq version $required_yq_version for $OS/$arch..."
    DOWNLOAD_URL="https://github.com/mikefarah/yq/releases/download/$required_yq_version/yq_${OS}_${arch}"
    if curl -Lo "$YQ_BIN" "$DOWNLOAD_URL" && chmod +x "$YQ_BIN"
    then
      echo "Successfully downloaded and installed yq version $required_yq_version for $arch."
      return
    else
      echo "Error: Failed to download yq version $required_yq_version for $arch."
    fi
  done

  echo "Error: Unable to install 'yq' for any of the supported architectures."
  exit 1
}