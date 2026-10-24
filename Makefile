.PHONY: generate generate-check lint test bench release ui-install ui-build ui-dev ui-check ui-bundle

generate:
	go generate ./...

# Fails when web/src/api/types.ts is out of date with ui/dto.go.
generate-check:
	cd ui && go generate .
	git diff --exit-code -- web/src/api/types.ts web/src/api/report.ts

lint:
	golangci-lint run --fix

test:
	go test -race -cover ./...

# Benchmarks, without tests. Compare two runs with benchstat:
#   make bench > old.txt; (change code); make bench > new.txt; benchstat old.txt new.txt
bench:
	go test -run '^$$' -bench . -benchmem -count 6 ./...

release:
	goreleaser build --snapshot

# --- web dashboard (requires bun: https://bun.sh) ---

ui-install:
	cd web && bun install --frozen-lockfile

ui-dev:
	cd web && bun run dev

ui-check:
	cd web && bun run lint && bun run typecheck && bun run test

ui-build:
	cd web && bun run build

# Creates the release asset served by the Go binary (see ui/bundle.go).
ui-bundle: ui-build
	tar -czf web/ninjabot-ui.tar.gz -C web/dist .
