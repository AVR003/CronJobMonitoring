.PHONY: run build frontend setup tidy dev

# First time on a new machine: creates the D:\ junction then installs packages.
setup:
	powershell -ExecutionPolicy Bypass -File setup.ps1

# Builds the React bundle (assumes setup has been run once).
frontend:
	cd frontend && npm run build

# Full production build: React bundle embedded in Go binary.
build: frontend
	go build -o monitoring-svc .

tidy:
	go mod tidy

run:
	go run .

# Development: Go backend + Vite hot-reload in parallel.
dev:
	go run . &
	cd frontend && npm run dev
