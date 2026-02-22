"""
End-to-end tests for safety protocols.
Tests automatic rollback on test failure and git checkpoint functionality.
"""

import json
import subprocess
from pathlib import Path
from typing import Generator

import pytest


@pytest.fixture
def built_cli(tmp_path: Path) -> Path:
    """Build the CLI binary and return its path."""
    cli_path = tmp_path / "reducto"

    result = subprocess.run(
        ["go", "build", "-o", str(cli_path), "./cmd/reducto"],
        capture_output=True,
        text=True,
    )

    if result.returncode != 0:
        raise RuntimeError(f"Failed to build CLI: {result.stderr}")

    return cli_path


@pytest.fixture
def project_with_tests(tmp_path: Path) -> Generator[Path, None, None]:
    """Create a project with code and passing tests."""
    (tmp_path / "main.py").write_text('''
def add(a: int, b: int) -> int:
    """Add two numbers."""
    return a + b

def multiply(a: int, b: int) -> int:
    """Multiply two numbers."""
    return a * b
''')

    (tmp_path / "test_main.py").write_text('''
import pytest
from main import add, multiply

def test_add():
    assert add(1, 2) == 3
    assert add(-1, 1) == 0
    assert add(0, 0) == 0

def test_multiply():
    assert multiply(2, 3) == 6
    assert multiply(-1, 5) == -5
    assert multiply(0, 100) == 0
''')

    subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "initial"], cwd=tmp_path, check=True, capture_output=True)

    yield tmp_path


@pytest.fixture
def project_with_failing_tests(tmp_path: Path) -> Generator[Path, None, None]:
    """Create a project with code and tests that will fail after a specific change."""
    (tmp_path / "calculator.py").write_text('''
def divide(a: int, b: int) -> float:
    """Divide two numbers."""
    if b == 0:
        raise ValueError("Cannot divide by zero")
    return a / b
''')

    (tmp_path / "test_calculator.py").write_text('''
import pytest
from calculator import divide

def test_divide_normal():
    assert divide(10, 2) == 5.0
    assert divide(9, 3) == 3.0

def test_divide_by_zero():
    with pytest.raises(ValueError):
        divide(1, 0)
''')

    (tmp_path / "requirements.txt").write_text("pytest>=7.0.0\n")

    subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "initial"], cwd=tmp_path, check=True, capture_output=True)

    yield tmp_path


class TestApplyDiffSafe:
    """Test apply_diff_safe with automatic rollback."""

    @pytest.mark.e2e
    def test_apply_diff_safe_success(self, project_with_tests: Path, built_cli: Path):
        """Test apply_diff_safe when tests pass."""
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        safe_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "apply_diff_safe",
            "params": {
                "path": "main.py",
                "diff": "--- a/main.py\n+++ b/main.py\n@@ -1,5 +1,5 @@\n def add(a: int, b: int) -> int:\n     \"\"\"Add two numbers.\"\"\"\n-    return a + b\n+    return a + b  # simple addition\n",
                "run_tests": True,
            }
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(project_with_tests)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{safe_req}\n", timeout=30)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response
            assert response["result"]["success"] is True
            assert response["result"]["tests_run"] is True
            assert response["result"]["tests_passed"] is True
            assert response["result"]["rolled_back"] is False
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_apply_diff_safe_rollback_on_failure(self, project_with_failing_tests: Path, built_cli: Path):
        """Test apply_diff_safe triggers rollback when tests fail."""
        breaking_diff = """--- a/calculator.py
+++ b/calculator.py
@@ -1,6 +1,4 @@
 def divide(a: int, b: int) -> float:
     \"\"\"Divide two numbers.\"\"\"
-    if b == 0:
-        raise ValueError("Cannot divide by zero")
     return a / b
"""

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        safe_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "apply_diff_safe",
            "params": {
                "path": "calculator.py",
                "diff": breaking_diff,
                "run_tests": True,
            }
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(project_with_failing_tests)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, stderr = proc.communicate(input=f"{init_req}\n{safe_req}\n", timeout=30)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response, f"No result in response: {response}, stderr: {stderr}"
            assert response["result"]["success"] is False, f"Response: {response}"
            assert response["result"]["tests_run"] is True
            assert response["result"]["tests_passed"] is False
            assert response["result"]["rolled_back"] is True

            original_content = (project_with_failing_tests / "calculator.py").read_text()
            assert "Cannot divide by zero" in original_content
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_apply_diff_safe_without_tests(self, project_with_tests: Path, built_cli: Path):
        """Test apply_diff_safe when run_tests is False."""
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        safe_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "apply_diff_safe",
            "params": {
                "path": "main.py",
                "diff": "--- a/main.py\n+++ b/main.py\n@@ -5,3 +5,4 @@\n def multiply(a: int, b: int) -> int:\n     \"\"\"Multiply two numbers.\"\"\"\n     return a * b\n+\n# End of file\n",
                "run_tests": False,
            }
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(project_with_tests)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{safe_req}\n", timeout=30)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response
            assert response["result"]["success"] is True
            assert response["result"]["tests_run"] is False
            assert response["result"]["tests_passed"] is True
        finally:
            proc.terminate()
            proc.wait()


class TestGitCheckpoint:
    """Test git checkpoint functionality."""

    @pytest.mark.e2e
    def test_checkpoint_creates_commit(self, project_with_tests: Path, built_cli: Path):
        """Test that checkpoint creates a git commit."""
        (project_with_tests / "new_file.py").write_text("# new file\n")

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        checkpoint_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "git_checkpoint",
            "params": {"message": "test checkpoint"}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(project_with_tests)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{checkpoint_req}\n", timeout=30)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response
            assert response["result"]["success"] is True
            assert "commit_hash" in response["result"]

            log_result = subprocess.run(
                ["git", "log", "--oneline", "-1"],
                cwd=project_with_tests,
                capture_output=True,
                text=True,
            )
            assert "test checkpoint" in log_result.stdout
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_rollback_restores_state(self, project_with_tests: Path, built_cli: Path):
        """Test that rollback restores previous state."""
        original_content = (project_with_tests / "main.py").read_text()

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        checkpoint_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "git_checkpoint",
            "params": {"message": "before change"}
        })
        diff_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 3,
            "method": "apply_diff",
            "params": {
                "path": "main.py",
                "diff": "--- a/main.py\n+++ b/main.py\n@@ -1,5 +1,4 @@\n-def add(a: int, b: int) -> int:\n-    \"\"\"Add two numbers.\"\"\"\n-    return a + b\n+\n"
            }
        })
        rollback_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 4,
            "method": "git_rollback",
            "params": {}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(project_with_tests)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(
                input=f"{init_req}\n{checkpoint_req}\n{diff_req}\n{rollback_req}\n",
                timeout=30
            )
            lines = stdout.strip().split("\n")

            rollback_response = json.loads(lines[3])
            assert "result" in rollback_response
            assert rollback_response["result"]["success"] is True

            restored_content = (project_with_tests / "main.py").read_text()
            assert restored_content == original_content
        finally:
            proc.terminate()
            proc.wait()


class TestMCPToolsListing:
    """Test that new tools are registered."""

    @pytest.mark.e2e
    def test_initialize_lists_apply_diff_safe(self, tmp_path: Path, built_cli: Path):
        """Test that initialize response includes apply_diff_safe tool."""
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(tmp_path)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n", timeout=10)
            response = json.loads(stdout.strip())

            assert "result" in response
            assert "tools" in response["result"]
            assert "apply_diff_safe" in response["result"]["tools"]
        finally:
            proc.terminate()
            proc.wait()
