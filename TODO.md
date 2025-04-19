# todo

## token handling removal
- [ ] **T032 · refactor · p0: remove all token counting, validation and handling**
    - **context:** Remove all token counting logic entirely from the application
    - **action:**
        1. Remove TokenManager interface and implementations
        2. Remove token counting from all providers
        3. Remove token validation logic from ModelProcessor
        4. Update orchestrator to not check tokens at all
        5. Let provider APIs handle their own token limits natively
    - **done-when:**
        1. All token handling code is removed
        2. Provider API calls have no token pre-checks
        3. All tests pass
    - **depends-on:** none

- [ ] **T033 · refactor · p0: update LLM interface to remove token-related methods**
    - **context:** Simplify provider interfaces by removing token-related functionality
    - **action:**
        1. Remove CountTokens method from LLMClient interface
        2. Remove GetModelInfo method from LLMClient interface
        3. Update all LLMClient implementations to match new interface
        4. Remove token-related structs (ProviderTokenCount, ProviderModelInfo)
    - **done-when:**
        1. LLMClient interface has no token-related methods
        2. All implementations are updated accordingly
        3. Tests pass
    - **depends-on:** [T032]

- [ ] **T034 · refactor · p0: remove token fields from registry schema**
    - **context:** Remove token-related fields from registry schema
    - **action:**
        1. Remove ContextWindow, MaxOutputTokens, Encoding fields from ModelDefinition
        2. Update registry tests to handle new schema
        3. Update any code using these fields
    - **done-when:**
        1. Registry schema has no token limit fields
        2. All code using these fields is updated
        3. All tests pass
    - **depends-on:** [T032, T033]

- [ ] **T035 · docs · p1: update documentation to reflect token handling removal**
    - **context:** Update documentation to explain token handling removal
    - **action:**
        1. Document that application no longer does token counting/validation
        2. Add notes that provider APIs handle their own limits natively
        3. Update error handling documentation for provider token limit errors
    - **done-when:**
        1. Documentation reflects new approach
        2. Error handling for provider limits is documented
    - **depends-on:** [T032, T033, T034]

## test infrastructure
- [x] **T031 · chore · p0: fix integration test failures and pre-commit hooks**
    - **context:** Testing Infrastructure Maintenance
    - **action:**
        1. Fix `internal/integration/rate_limit_test.go` - undefined methods and type issues
        2. Update `internal/integration/test_utils.go` - remove unused imports and ensure types
        3. Ensure all provider mock objects are properly initialized in TestEnv
        4. Fix API client type mismatches and parameter issues
    - **done-when:**
        1. All integration tests pass.
        2. Pre-commit hooks pass without --no-verify.
        3. `go vet` and `go build` run without errors.
    - **depends-on:** none

## registry & configuration
- [x] **T001 · refactor · p1: remove absolute path fallback in registry config lookup**
    - **context:** CR-01: Registry Config Path: Kill Absolute Fallbacks
    - **action:**
        1. Remove hardcoding of `/Users/phaedrus/Development/architect/config/models.yaml` in `internal/registry/manager.go`.
        2. Log and return clear error if default config missing.
    - **done-when:**
        1. Code referencing absolute paths is removed.
        2. Tests pass demonstrating error handling for missing config.
        3. CLI returns informative error if config is missing.
    - **depends-on:** none

- [x] **T002 · test · p2: update registry tests to use injected config paths**
    - **context:** CR-01: Registry Config Path: Kill Absolute Fallbacks
    - **action:**
        1. Refactor tests in `internal/registry/manager_test.go` to inject configuration paths or use temporary test files.
        2. Ensure tests cover scenarios where config file exists and where it is missing.
    - **done-when:**
        1. Registry tests no longer rely on hardcoded paths.
        2. Tests pass for both found and missing config file scenarios.
    - **depends-on:** [T001]

- [x] **T003 · refactor · p1: centralize model/provider detection in registry**
    - **context:** CR-06: Model Detection: Config-driven Not String Matching
    - **action:**
        1. Implement functions in `internal/registry/manager.go` to determine provider based on model name using registry data.
        2. Ensure these functions handle unknown models gracefully (e.g., return error or specific type).
    - **done-when:**
        1. Registry provides functions like `GetProviderForModel(modelName)`.
        2. Logic relies solely on loaded registry configuration.
    - **depends-on:** none

- [x] **T004 · test · p2: add tests for registry model detection**
    - **context:** CR-06: Model Detection: Config-driven Not String Matching
    - **action:**
        1. Add unit tests in `internal/registry/manager_test.go` to verify the model/provider detection logic.
        2. Include test cases for known models, unknown models, and potentially malformed model names.
    - **done-when:**
        1. Tests pass covering various detection scenarios.
    - **depends-on:** [T003]

- [x] **T005 · refactor · p2: refactor CLI code to use registry for model detection**
    - **context:** CR-06: Model Detection: Config-driven Not String Matching
    - **action:**
        1. Update `cmd/architect/cli.go` validation logic to use `registry.GetProviderForModel`.
        2. Update `internal/architect/app.go` or orchestrator logic to use registry detection instead of string matching.
    - **done-when:**
        1. CLI and core app logic use registry for provider detection.
        2. String matching for provider detection is removed.
    - **depends-on:** [T003]

## security & logging
- [x] **T006 · test · p0: audit logger calls for secret leaks in registry**
    - **context:** CR-02: Secrets: Audit & Seal Log Leaks
    - **action:**
        1. Review all logger calls in `internal/registry/*` for potential leaks of API keys or other sensitive info.
    - **done-when:**
        1. Audit complete and documented.
    - **depends-on:** none

- [x] **T007 · test · p0: audit logger calls for secret leaks in openrouter provider**
    - **context:** CR-02: Secrets: Audit & Seal Log Leaks
    - **action:**
        1. Review all logger calls in `providers/openrouter/*` for potential leaks of API keys or other sensitive info.
    - **done-when:**
        1. Audit complete and documented.
    - **depends-on:** none

- [x] **T008 · refactor · p0: refactor logging calls to avoid outputting secrets**
    - **context:** CR-02: Secrets: Audit & Seal Log Leaks
    - **action:**
        1. Modify logger calls identified in audits to log only presence/absence of secrets, never values.
        2. Ensure error messages returned from providers do not inadvertently contain secrets before logging.
    - **done-when:**
        1. Code refactored to prevent secret logging.
        2. Manual code review confirms no secrets are logged.
    - **depends-on:** [T006, T007]

- [x] **T009 · test · p0: add unit test to detect secrets in logs**
    - **context:** CR-02: Secrets: Audit & Seal Log Leaks
    - **action:**
        1. Create a test helper or modify logger mock to fail tests if a pattern resembling a secret (e.g., API key format) is logged.
        2. Integrate this check into relevant unit tests for providers and registry.
    - **done-when:**
        1. Unit tests fail if secrets are logged.
        2. Relevant tests pass with the new check incorporated.
    - **depends-on:** [T008]

## error handling
- [x] **T010 · refactor · p1: create shared errors package**
    - **context:** CR-03: Error Handling: Deduplicate & Centralize
    - **action:**
        1. Create `internal/llm/errors.go` (or similar).
        2. Define shared error types/structs and categorization logic (e.g., rate limit, auth, invalid request).
    - **done-when:**
        1. Shared errors package exists with core definitions.
        2. Basic structure for categorization logic is in place.
    - **depends-on:** none

- [x] **T011 · refactor · p2: refactor openrouter provider errors to use shared package**
    - **context:** CR-03: Error Handling: Deduplicate & Centralize
    - **action:**
        1. Modify `providers/openrouter/errors.go` to use types and logic from `internal/llm/errors.go`.
        2. Remove duplicated error definitions/logic.
    - **done-when:**
        1. OpenRouter provider uses the shared errors package.
        2. Tests for OpenRouter error handling pass.
    - **depends-on:** [T010]

- [x] **T012 · refactor · p2: refactor openai provider errors to use shared package**
    - **context:** CR-03: Error Handling: Deduplicate & Centralize
    - **action:**
        1. Modify `providers/openai/errors.go` to use types and logic from `internal/llm/errors.go`.
        2. Remove duplicated error definitions/logic.
    - **done-when:**
        1. OpenAI provider uses the shared errors package.
        2. Tests for OpenAI error handling pass.
    - **depends-on:** [T010]

- [x] **T013 · refactor · p2: refactor gemini provider errors to use shared package**
    - **context:** CR-03: Error Handling: Deduplicate & Centralize
    - **action:**
        1. Modify `providers/gemini/errors.go` to use types and logic from `internal/llm/errors.go`.
        2. Remove duplicated error definitions/logic.
    - **done-when:**
        1. Gemini provider uses the shared errors package.
        2. Tests for Gemini error handling pass.
    - **depends-on:** [T010]

- [x] **T014 · test · p2: add tests for shared error handling logic**
    - **context:** CR-03: Error Handling: Deduplicate & Centralize
    - **action:**
        1. Create `internal/llm/errors_test.go`.
        2. Add unit tests verifying the error categorization logic for various error patterns/types.
    - **done-when:**
        1. Shared error logic has sufficient test coverage.
        2. Tests pass.
    - **depends-on:** [T010]

## providers/openrouter
- [x] **T015 · refactor · p1: remove string matching for token logic in openrouter client**
    - **context:** CR-04: Token Logic: Eliminate String Matching Hacks
    - **action:**
        1. Remove any code in `providers/openrouter/client.go` that uses string matching on model names to determine token limits or tokenizer encoding.
    - **done-when:**
        1. String matching logic for token properties is removed.
    - **depends-on:** none

- [x] **T016 · refactor · p1: refactor openrouter generatecontent to use request-local params**
    - **context:** CR-05: Parameter Mapping: Remove Shared State
    - **action:**
        1. Modify `GenerateContent` in `providers/openrouter/client.go` to copy request parameters to local variables.
        2. Ensure no parameters are read directly from receiver fields within the request handling logic.
    - **done-when:**
        1. `GenerateContent` uses only method-local variables for request parameters.
    - **depends-on:** none

- [x] **T017 · refactor · p1: remove receiver field mutations for request params in openrouter client**
    - **context:** CR-05: Parameter Mapping: Remove Shared State
    - **action:**
        1. Identify and remove any code in `providers/openrouter/client.go` that modifies receiver fields (e.g., `c.temperature = ...`) based on request parameters.
    - **done-when:**
        1. No receiver fields are mutated based on request-specific parameters.
    - **depends-on:** [T016]

- [x] **T018 · test · p1: add race detector test for concurrent openrouter requests**
    - **context:** CR-05: Parameter Mapping: Remove Shared State
    - **action:**
        1. Create a new test in `providers/openrouter/client_test.go` that makes concurrent calls to `GenerateContent`.
        2. Run this test with the `-race` flag enabled.
    - **done-when:**
        1. Concurrent request test exists.
        2. Test passes under the race detector (`go test -race ./...`).
    - **depends-on:** [T017]

## other providers
- [x] **T019 · refactor · p1: remove string matching for token logic in openai client**
    - **context:** CR-04: Token Logic: Eliminate String Matching Hacks
    - **action:**
        1. Remove any code in `providers/openai/client.go` that uses string matching on model names to determine token limits or tokenizer encoding.
    - **done-when:**
        1. String matching logic for token properties is removed.
    - **depends-on:** none

- [x] **T020 · refactor · p1: remove string matching for token logic in gemini client**
    - **context:** CR-04: Token Logic: Eliminate String Matching Hacks
    - **action:**
        1. Remove any code in `providers/gemini/client.go` that uses string matching on model names to determine token limits or tokenizer encoding.
    - **done-when:**
        1. String matching logic for token properties is removed.
    - **depends-on:** none

## model info & token counting
- [~] **T021 · feature · p1: implement fetching token limits/encodings exclusively from registry**
    - **context:** CR-04: Token Logic: Eliminate String Matching Hacks
    - **status:** OBSOLETE - Superseded by T032, T033, T034 that simplify token handling
    - **action:**
        1. ~~Modify provider clients (`providers/*`) to retrieve token limits and encoding information only from the registry manager.~~
        2. ~~Ensure this data is accessed correctly during token counting and request preparation.~~
    - **done-when:**
        1. ~~All provider clients use registry data for token limits/encodings.~~
        2. ~~Tests relying on this logic pass.~~
    - **depends-on:** [T015, T019, T020, T003]

- [~] **T022 · feature · p1: implement fast-fail for unknown models in token logic**
    - **context:** CR-04: Token Logic: Eliminate String Matching Hacks
    - **status:** OBSOLETE - Superseded by T032, T033, T034 that simplify token handling
    - **action:**
        1. ~~Modify provider clients (`providers/*`) to explicitly check if a model exists in the registry before attempting token operations.~~
        2. ~~Return a clear error and log if the model is not found in the configuration.~~
    - **done-when:**
        1. ~~Clients fail fast with clear errors for unknown models during token operations.~~
        2. ~~Tests cover the unknown model scenario.~~
    - **depends-on:** [T021]

## testing
- [ ] **T023 · test · p2: add table-driven tests for openrouter error/edge cases**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Add table-driven tests in `providers/openrouter/*_test.go` covering API errors, HTTP errors, invalid inputs, streaming issues, etc.
    - **done-when:**
        1. Comprehensive table-driven tests for error/edge cases exist and pass.
    - **depends-on:** [T011]

- [ ] **T024 · test · p2: add table-driven tests for openai error/edge cases**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Add table-driven tests in `providers/openai/*_test.go` covering API errors, HTTP errors, invalid inputs, streaming issues, etc.
    - **done-when:**
        1. Comprehensive table-driven tests for error/edge cases exist and pass.
    - **depends-on:** [T012]

- [ ] **T025 · test · p2: add table-driven tests for gemini error/edge cases**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Add table-driven tests in `providers/gemini/*_test.go` covering API errors, HTTP errors, invalid inputs, streaming issues, etc.
    - **done-when:**
        1. Comprehensive table-driven tests for error/edge cases exist and pass.
    - **depends-on:** [T013]

- [ ] **T026 · test · p2: add tests for config bootstrapping error/edge cases**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Add tests covering registry initialization errors (e.g., invalid YAML, missing file after install attempt).
        2. Add tests for CLI flag parsing errors related to config/registry.
    - **done-when:**
        1. Tests pass for config bootstrapping error scenarios.
    - **depends-on:** [T001, T002]

- [ ] **T027 · test · p2: add tests for audit logging error/edge cases**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Add tests for `internal/auditlog/*` covering file write errors, JSON marshal errors, and concurrent logging.
    - **done-when:**
        1. Tests pass for audit logging error scenarios.
    - **depends-on:** none

- [ ] **T028 · test · p2: ensure error path test coverage > 90% for all providers**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Review test coverage reports for `providers/*/errors.go` and related error handling code.
        2. Add necessary tests to achieve >= 90% coverage for error paths.
    - **done-when:**
        1. Test coverage for provider error handling meets target.
        2. CI coverage check passes.
    - **depends-on:** [T011, T012, T013, T014]

- [ ] **T029 · chore · p2: enforce 90%+ test coverage in CI**
    - **context:** CR-07: Test Coverage: Error & Edge Paths
    - **action:**
        1. Configure CI workflow (e.g., GitHub Actions) to run code coverage checks.
        2. Set the failure threshold to 90% for relevant packages/overall coverage.
    - **done-when:**
        1. CI pipeline includes a coverage check step.
        2. CI fails if coverage drops below 90%.
    - **depends-on:** [T028]

## documentation
- [ ] **T030 · feature · p2: add project-specific OpenRouter integration documentation**
    - **context:** Documentation improvements
    - **action:**
        1. Write detailed doc for OpenRouter integration, covering model config, API key handling, registry mapping, parameter mapping, and error handling.
    - **done-when:**
        1. OpenRouter docs present and accurate; referenced in README.
    - **depends-on:** none
