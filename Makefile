.PHONY: publish build lint example-queries dev format test bench docs-serve docs-build e2e audit audit-backend audit-docker audit-secrets install-tools sonar

DNS_PORT ?= 53
WEBSITE_PORT ?= 8080
GHCR_REPO ?= ghcr.io/rasty94/goaway
LATEST_VERSION = $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "latest")

# --- Security & Optimization ---
TRIVY = trivy
GOSEC = gosec
GOVULNCHECK = govulncheck
SONAR_SCANNER = sonar-scanner

install-tools:
	@echo "🛠️ Installing Security Audit Tools..."
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest

audit: audit-backend audit-docker audit-secrets

audit-backend:
	@echo "🔍 Running Backend Security Audit (Code AST)..."
	@$(GOSEC) -quiet ./backend/...
	@echo "📦 Checking for known vulnerabilities in Go dependencies..."
	@$(GOVULNCHECK) ./...

audit-docker:
	@echo "🚢 Auditing Docker configuration and vulnerabilities..."
	@$(TRIVY) config --severity HIGH,CRITICAL .
	@echo "🐳 Scanning production image for CVEs..."
	@$(TRIVY) image --severity HIGH,CRITICAL ${GHCR_REPO}:latest

audit-secrets:
	@echo "🔑 Scanning for hardcoded secrets..."
	@gitleaks detect --source . --verbose --redact || echo "⚠️ Gitleaks scan failed or not found. Install with: brew install gitleaks"

sonar:
	@echo "🧪 Generating Code Coverage for Sonar..."
	@go test -coverprofile=coverage.out ./backend/...
	@echo "📡 Launching SonarQube Scanner..."
	@$(SONAR_SCANNER) -Dsonar.login=${SONAR_TOKEN} || echo "⚠️ Sonar Scanner failed. Make sure SONAR_TOKEN is set and scanner is in PATH."

# --- Cycle & Development ---

publish:
	docker buildx create --name multiarch-builder --use || true

	docker buildx build \
	--platform linux/amd64,linux/arm64/v8 \
	--file Dockerfile.multi \
	--tag ${GHCR_REPO}:${LATEST_VERSION} \
	--tag ${GHCR_REPO}:latest \
	--push \
	.

	docker buildx rm multiarch-builder || true

ghcr-publish:
	./.agents/skills/ghcr-publish/scripts/publish.sh


build: ; pnpm -C client install && pnpm -C client build
start: ; docker compose up -d

format:			; npx prettier --write "client/**/*.{html,css,js,tsx}"
install:		; pnpm -C client install
lint:			; @echo "🧹 Running Frontend Lints..." && pnpm -C client lint && echo "🧹 Running Backend Lints..." && golangci-lint run ./backend/...
commitlint:  	; pnpm -C client commitlint

dev: build
	docker compose -f docker-compose.dev.yml up

dev-website:   ; pnpm -C client install && pnpm -C client dev
dev-server:    ; mkdir client/dist ; touch client/dist/.fake ; air .

test: install lint commitlint
	go test -count=1 -race ./test/...

e2e:			; ./test/e2e/run.sh

bench: 		   ; go run test/benchmark.go -test.bench=.
bench-profile: ; go run test/benchmark.go -test.bench=. & go tool pprof http://localhost:6060/debug/pprof/profile\?seconds\=5

docs-serve: ; make -C docs serve
docs-build: ; make -C docs build
