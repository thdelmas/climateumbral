# Vite + Vue frontend, Go API, all through Docker.
# The static prototype (prototype/index.html) still needs no build at all.

.PHONY: help dev down fetch build run prototype

help:
	@echo "Tilewhip targets:"
	@echo "  make dev        dev stack: Vite (:5173) + Go API (:8080)"
	@echo "  make down       stop the dev stack"
	@echo "  make fetch      (analysis) fetch a static grid into data/"
	@echo "  make build      build the production image (Go serves web)"
	@echo "  make run        run the production image at http://localhost:8080"
	@echo "  make prototype  open the original no-build prototype"

dev:
	docker compose up --build

down:
	docker compose down

fetch:
	docker compose --profile tools run --rm fetch

build:
	docker build -t tilewhip .

run:
	docker run --rm -p 127.0.0.1:8080:8080 \
		-v "$(PWD)/data:/data" tilewhip

prototype:
	xdg-open prototype/index.html
