#!/bin/bash

echo "Downloading frontend config from operator repo"
curl "https://raw.githubusercontent.com/netobserv/network-observability-operator/refs/heads/main/controllers/consoleplugin/config/static-frontend-config.yaml" -o ./cmd/config.yaml