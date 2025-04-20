# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

* **Build:** `go build ./...`
* **Run Tests:** `go test ./...`
* **Run Single Test:** `go test -v -run TestName ./path/to/package`
* **Run E2E Tests:** `./internal/e2e/run_e2e_tests.sh [-v] [-r TestPattern]`
* **Check Coverage:** `go test -cover ./...` or `go test -coverprofile=coverage.out ./...`
* **Format Code:** `go fmt ./...`
* **Lint Code:** `go vet ./...`

## Go Style Guidelines

* **Package Structure:** Package-by-feature, with small focused interfaces
* **Imports:** Group standard library, external, internal imports with blank line separators
* **Error Handling:** Return errors rather than panic; use structured errors with context
* **Naming:** Use clear, descriptive names; camelCase for variables, PascalCase for exports
* **Testing:** Write tests first (TDD); use table-driven tests; focus on integration testing
* **Types:** Use strong typing; avoid unnecessary interface{}/any; leverage Go's type system
* **Error Flow:** Explicit error handling; no suppression of errors or linter warnings
* **Comments:** Document *why*, not *how*; code should be self-documenting
* **Validation:** Validate all external input rigorously at system boundaries

## Mandatory Practices

* **Use TDD:** Write tests first, make them fail, then implement code to pass
* **Conventional Commits:** Follow the spec for automated versioning/changelogs
* **No Secrets in Code:** Use environment variables or designated secret managers
* **Structured Logging:** Use the project's standard structured logging library
* **Pre-commit Quality:** All code must pass tests, lint, and format checks
* **Cross-Package Testing:** Focus on robust integration tests over unit tests

## Using the `architect` CLI

This repo contains the `architect` CLI tool itself, which can analyze code using different LLM models. When working on problems:

1. For complex tasks, outline your approach first
2. If stuck, consider using `architect` to get a different perspective
3. Example: `architect --instructions temp_instructions.txt ./path/to/relevant/files`

The tool works by creating a temporary file with instructions, then analyzing the specified paths. API keys are pre-configured locally.
