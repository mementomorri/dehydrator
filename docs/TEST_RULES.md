# Test Rules

Following the principles of Test-Driven Development (TDD) for the Semantic Compression Engine, the following test suite focuses on functional behavior and user-facing requirements. These tests verify that the application correctly interacts with the filesystem, models, and version control without assuming any specific internal implementation.

## 1. Repository Analysis and Context Mapping

### Test Case: Initial Project Mapping

**Scenario**: Run the tool in a directory containing multiple modules and shared utilities.
**Expectation**: The tool must identify key classes, function signatures, and dependencies across the project. It should detect the project’s main entry points and export a summary or "map".

### Test Case: Language Recognition

**Scenario**: Run the tool on a multi-language repository (e.g., Python and JavaScript).
**Expectation**: The tool must correctly detect the syntax of both languages and apply the appropriate language-specific parsing for each file type.

## 2. Semantic Compression and Refactoring

### Test Case: Cross-File Deduplication Detection

**Scenario**: Provide two files with semantically identical logic (e.g., identical input validation blocks) but different variable names.
**Expectation**: The tool must identify these as redundant and suggest a refactoring plan to extract the logic into a shared utility.

### Test Case: Idiomatic Transformation (Pythonic Alignment)

**Scenario**: Run the tool on a file containing verbose procedural code (e.g., a multi-line for loop used for list creation).
**Expectation**: The tool should propose replacing the block with a single-line list comprehension or a standard library equivalent.

### Test Case: Design Pattern Injection

**Scenario**: Run the tool on a file with complex, deeply nested if-else conditionals.
**Expectation**: The tool must suggest a specific design pattern, such as the Strategy or Factory pattern, to simplify the branching logic.

## 3. Safety Protocols and Git Integration

### Test Case: Git-Native Checkpointing

**Scenario**: Initiate a refactoring session on a project with uncommitted changes.
**Expectation**: The tool must either warn the user to commit or stash current changes.

### Test Case: Automatic Rollback on Test Failure

**Scenario**: The tool applies a refactor that causes an existing project test (e.g., pytest or npm test) to fail.
**Expectation**: Upon detecting a non-zero exit code from the test runner, the tool must warn the user and suggest reverting the file changes to the previous Git state before proceeding.

### Test Case: Human-in-the-Loop Approval

**Scenario**: User requests a compression operation.
**Expectation**: The tool must present a "Plan" mode showing a side-by-side diff of proposed changes and wait for a user confirmation (e.g., "y/n") before editing files in-place. Or proceed without approval if command flag with pre-approval is set.

## 4. Model Orchestration and Performance

### Test Case: Model Provider Switching

**Scenario**: Configure the tool to use a local Ollama instance for simple tasks and a remote Claude model for architectural planning, if both enabled.
**Expectation**: The tool must correctly route requests based on the user's config.yaml or CLI flags, ensuring the local model handles small-scope edits while the remote model handles multi-file reasoning.

### Test Case: Functional Parity Validation (Pass@1)

**Scenario**: Apply a refactor to a core business logic function.
**Expectation**: After the refactor, the function must pass 100% of its original unit tests to ensure no regression in external behavior.

## 5. Reporting and Metrics

### Test Case: Complexity Reduction Report

**Scenario**: Execute the tool with the --report flag.
**Expectation**: The tool must generate a document (Markdown) summarizing the delta in Lines of Code (LOC), Cyclomatic Complexity, and "Cognitive Complexity" scores before and after the session.

### Test Case: Duplicate Removal Statistics

**Scenario**: Run a deduplication session across a large project.
**Expectation**: The final report must specify exactly which redundant blocks were removed and how much total "Technical Debt" volume was eliminated.

## 6. User-Flow Integration

### Test Case: CLI Flow Continuity

**Scenario**: Navigate to a project folder and run dehydrate --deduplicate.
**Expectation**: The application must successfully complete the loop: Scan -> Propose Plan -> User approves (skip if flag set to pre-approve) -> Edit In-Place -> Run Tests -> Commit to Git (Optional).
