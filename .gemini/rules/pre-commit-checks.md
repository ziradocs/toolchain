---
name: pre-commit-checks
description: Mandates running linters and tests before executing a git commit.
---

# Rule: Pre-Commit Checks

Whenever the user asks you to make a `git commit`, or when you determine a commit is necessary to complete a task, you **MUST** adhere to the following workflow:

1. **Run Linters:** Execute the project's linter on all modified modules (e.g., `golangci-lint run ./...` in Go modules).
2. **Run Tests:** Execute the project's test suite on all modified modules (e.g., `go test ./...`).
3. **Verify Success:** Ensure that both the linter and tests exit with a status code of `0`.
4. **Fix Issues:** If any step fails, you must attempt to fix the code, or ask the user for guidance if the fix is not obvious, **before** proceeding to commit.
5. **Commit:** Only execute the `git commit` command after you have confirmed that both the linter and tests pass.
