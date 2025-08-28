##@ dev helpers

.PHONY:	flows-mock
flows-mock: build ## Run flow capture using mocks
	./build/network-observability-cli get-flows --mock true
	tput reset

.PHONY:	packets-mock
packets-mock: build ## Run packet capture using mocks
	./build/network-observability-cli get-packets --mock true
	tput reset

.PHONY:	metrics-mock
metrics-mock: build ## Run metrics capture using mocks
	./build/network-observability-cli get-metrics --mock true
	tput reset