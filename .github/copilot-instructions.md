# GitHub Copilot & Workspace Agent Instructions

1. **Agentic Skills Priority**: Read `.aipods/skills/` before starting any coding task.
2. **Issue Creation Mandatory**: Never write code or specs without an open GitHub Issue with labels (`feature`, `bug`, `spec-approved`).
3. **Spec-Driven Development (SDD)**: All architectural features must be specified before implementation.
4. **No Hardcoded Paths**: Ensure all file references and script executions use relative paths.
5. **Quality Gate Verification**: Run `bash scripts/deploy_stack.sh` to test Go backend, React frontends, and security linters.
