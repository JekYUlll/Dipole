.DEFAULT_GOAL := help
GO ?= go
NPM ?= npm
DOCKER ?= docker
IMAGE_TAG ?= latest
SERVICES := core gateway message sync search search-indexer migrate
TOOLS := $(notdir $(wildcard cmd/tools/*))
REVISION := $(shell git rev-parse HEAD)
CREATED := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
DIRTY := $(if $(shell git status --porcelain),true,false)
META = --build-arg DIPOLE_VCS_REVISION=$(REVISION) --build-arg DIPOLE_BUILD_CREATED=$(CREATED)

.PHONY: help build images agent frontend legacy-image $(addprefix build-,$(SERVICES)) $(addprefix image-,$(SERVICES)) $(addprefix tool-,$(TOOLS)) image-agent image-agent-timeline-repair
help:
	@printf '%s\n' 'make build / images       Main Go binaries / images' 'make image-core           Rebuild one service and its image' 'make agent / frontend     Compile TypeScript / web assets' 'make image-agent          Build Agent container' 'make tool-<name>          Build an optional cmd/tools program' 'make legacy-image         Optional all-in-one benchmark image' 'just --list               Development commands'

build: $(addprefix build-,$(SERVICES))
images: $(addprefix image-,$(SERVICES))

# Always invoke Go: its build cache tracks embedded assets and build options too.
define service
.PHONY: build-$(1) image-$(1)
build-$(1):
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux $(GO) build $(GO_BUILD_FLAGS) -o dist/$(2) ./cmd/$(3)
image-$(1): build-$(1)
	$(DOCKER) build -f deploy/images/go-service.Dockerfile -t "$$(or $$(DIPOLE_$(4)_IMAGE),dipole-$(1):$$(IMAGE_TAG))" --build-arg DIPOLE_BINARY=$(2) $$(META) --build-arg DIPOLE_BUILD_DIRTY=$$(DIRTY) .
endef
$(eval $(call service,core,dipole-server,services/core,CORE))
$(eval $(call service,gateway,dipole-gateway,services/gateway,GATEWAY))
$(eval $(call service,message,dipole-message,services/message,MESSAGE))
$(eval $(call service,sync,dipole-sync,services/sync,SYNC))
$(eval $(call service,search,dipole-search,services/search,SEARCH))
$(eval $(call service,search-indexer,dipole-search-indexer,services/search-indexer,SEARCH_INDEXER))
$(eval $(call service,migrate,dipole-migrate,tools/migrate,MIGRATE))
$(eval $(call service,agent-timeline-repair,dipole-agent-task-timeline-repair,tools/agent-task-timeline-repair,AGENT_TIMELINE_REPAIR))

$(addprefix tool-,$(TOOLS)): tool-%:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux $(GO) build $(GO_BUILD_FLAGS) -o dist/dipole-$* ./cmd/tools/$*

agent:
	$(NPM) --prefix services/agent-runtime run build
frontend:
	$(NPM) --prefix frontend run build
image-agent:
	$(DOCKER) build -t "$(or $(DIPOLE_AGENT_IMAGE),dipole-agent:$(IMAGE_TAG))" $(META) --build-arg DIPOLE_VCS_DIRTY=$(DIRTY) services/agent-runtime

# Historical multi-binary fixtures still use /app/dipole-* entrypoints.
legacy-image: frontend build $(addprefix tool-,$(filter-out migrate,$(TOOLS)))
	$(DOCKER) build -t "$(or $(IMAGE_NAME),dipole-server):$(IMAGE_TAG)" $(META) --build-arg DIPOLE_VCS_DIRTY=$(DIRTY) .
