# Vite + Vue frontend, Go API, all through Docker.
# The static prototype (prototype/index.html) still needs no build at all.

.PHONY: help dev down fetch build run prototype

help:
	@echo "Tilewhip targets:"
	@echo "  make dev        dev stack: Vite (:5173) + Go API (:8080)"
	@echo "  make down       stop the dev stack"
	@echo "  make fetch      fetch the grid into data/ (skips if already there)"
	@echo "  make build      build the production image (Go serves web)"
	@echo "  make run        run the production image at http://localhost:8080"
	@echo "  make prototype  open the original no-build prototype"

dev:
	docker compose up --build

down:
	docker compose down

fetch:
	docker compose run --rm fetch

build:
	docker build -t tilewhip .

run:
	docker run --rm -p 8080:8080 -v $(PWD)/data:/data tilewhip

prototype:
	xdg-open prototype/index.html
