PROFILE ?= netbox-4.7
TOFU ?= $(shell command -v tofu)
PKGS ?= ./internal/plugins/...

.PHONY: build test lint generate check-generate testenv-up testenv-down testacc testacc-terraform plugin

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

# Regenerates plugin descriptors and docs.
generate:
	go generate ./...

# Fails when a generated file is out of date (success criterion SC12).
check-generate: generate
	git diff --exit-code -- internal/plugins docs

# Starts NetBox with the plugins under test: make testenv-up PROFILE=netbox-4.6
testenv-up:
	./testenv/up.sh $(PROFILE)

testenv-down:
	./testenv/down.sh $(PROFILE)

# Acceptance tests with OpenTofu against the running profile. Packages run one
# at a time: they share one NetBox server.
testacc:
	. testenv/.env.$(PROFILE) && TF_ACC=1 TF_ACC_TERRAFORM_PATH=$(TOFU) TF_ACC_PROVIDER_HOST=registry.opentofu.org \
		go test -p 1 $(PKGS) -run TestAcc -count=1 -timeout 60m -v

# Acceptance tests with Terraform, downloaded by terraform-plugin-testing
# unless TF_ACC_TERRAFORM_PATH is set.
testacc-terraform:
	. testenv/.env.$(PROFILE) && TF_ACC=1 go test -p 1 $(PKGS) -run TestAcc -count=1 -timeout 60m -v

# Starts a plugin folder:
#   make plugin NAME=bgp PACKAGE=netbox-bgp STATUS_KEY=netbox_bgp API_PREFIX=bgp
plugin:
	@test -n "$(NAME)" -a -n "$(PACKAGE)" -a -n "$(STATUS_KEY)" -a -n "$(API_PREFIX)" || \
		{ echo "usage: make plugin NAME=<folder> PACKAGE=<pip package> STATUS_KEY=<key in /api/status/> API_PREFIX=<prefix>"; exit 2; }
	@test ! -e internal/plugins/$(NAME) || { echo "internal/plugins/$(NAME) exists"; exit 1; }
	@mkdir -p internal/plugins/$(NAME)
	@printf '%s\n' \
		'# $(PACKAGE)' \
		'name: $(NAME)' \
		'title: ""' \
		'package: $(PACKAGE)' \
		'status_key: $(STATUS_KEY)' \
		'api_prefix: $(API_PREFIX)' \
		'maturity: experimental' \
		'compatibility:' \
		'  - netbox: ">= 4.7.0, < 4.8.0"' \
		'    plugin: ">= 0.0.0"' \
		'models:' \
		'  - endpoint: <endpoint>' \
		'    name: <resource suffix>' > internal/plugins/$(NAME)/plugin.yaml
	@printf '%s\n' '// Package $(NAME) is the provider folder for the $(PACKAGE) plugin.' 'package $(NAME)' '' \
		'//go:generate go run ../../../tools/nbgen' > internal/plugins/$(NAME)/generate.go
	@echo "Created internal/plugins/$(NAME). Next steps are in CONTRIBUTING.md."
