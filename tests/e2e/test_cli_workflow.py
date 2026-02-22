"""
End-to-end tests for full CLI workflow.
Tests the complete analyze -> deduplicate -> commit cycle.
"""

import json
import os
import subprocess
from pathlib import Path
from typing import Generator

import pytest


@pytest.fixture
def built_cli(tmp_path: Path) -> Path:
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
def git_project_with_duplicates(tmp_path: Path) -> Generator[Path, None, None]:
    (tmp_path / "auth.py").write_text('''
def validate_email(email):
    """Validate email address."""
    if not email:
        raise ValueError("Email required")
    if '@' not in email:
        raise ValueError("Invalid email format")
    if len(email) > 255:
        raise ValueError("Email too long")
    return email.lower().strip()

def validate_password(password):
    """Validate password."""
    if not password:
        raise ValueError("Password required")
    if len(password) < 8:
        raise ValueError("Password too short")
    return password
''')

    (tmp_path / "user.py").write_text('''
def check_email_address(email_addr):
    """Check email address."""
    if not email_addr:
        raise Exception("Email is required")
    if '@' not in email_addr:
        raise Exception("Email format is invalid")
    if len(email_addr) > 255:
        raise Exception("Email address too long")
    return email_addr.lower().strip()

def check_password(pwd):
    """Check password."""
    if not pwd:
        raise Exception("Password is required")
    if len(pwd) < 8:
        raise Exception("Password is too short")
    return pwd
''')

    subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "initial"], cwd=tmp_path, check=True, capture_output=True)

    yield tmp_path


@pytest.fixture
def non_idiomatic_python_project(tmp_path: Path) -> Generator[Path, None, None]:
    (tmp_path / "data.py").write_text('''
def filter_positive(numbers):
    """Filter positive numbers - non-idiomatic."""
    result = []
    for n in numbers:
        if n > 0:
            result.append(n)
    return result

def greet(name, age):
    """Greet user - non-idiomatic string formatting."""
    return "Hello, " + name + "! You are " + str(age) + " years old."

def read_file_lines(filepath):
    """Read file lines - non-idiomatic file handling."""
    f = open(filepath, "r")
    try:
        return f.readlines()
    finally:
        f.close()
''')

    subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "initial"], cwd=tmp_path, check=True, capture_output=True)

    yield tmp_path


@pytest.fixture
def complex_conditional_project(tmp_path: Path) -> Generator[Path, None, None]:
    (tmp_path / "payment.py").write_text('''
def process_payment(payment_type, amount, currency):
    """Process payment based on type, amount, and currency."""
    if payment_type == "credit_card":
        if currency == "USD":
            if amount > 10000:
                return {"type": "high_value_cc_usd"}
            else:
                return {"type": "standard_cc_usd"}
        elif currency == "EUR":
            return {"type": "cc_eur"}
    elif payment_type == "paypal":
        if currency == "USD":
            return {"type": "paypal_usd"}
        elif currency == "EUR":
            return {"type": "paypal_eur"}
    elif payment_type == "bank_transfer":
        if amount > 50000:
            return {"type": "wire"}
        else:
            return {"type": "ach"}
    else:
        raise ValueError("Unsupported payment type")
''')

    subprocess.run(["git", "init"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "config", "user.name", "Test User"], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "add", "."], cwd=tmp_path, check=True, capture_output=True)
    subprocess.run(["git", "commit", "-m", "initial"], cwd=tmp_path, check=True, capture_output=True)

    yield tmp_path


class TestCLICommands:
    """Test basic CLI commands."""

    @pytest.mark.e2e
    def test_version_command(self, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "version"],
            capture_output=True,
            text=True,
        )

        assert result.returncode == 0
        assert "reducto" in result.stdout
        assert "v0.1.0" in result.stdout

    @pytest.mark.e2e
    def test_help_command(self, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "--help"],
            capture_output=True,
            text=True,
        )

        assert result.returncode == 0
        assert "analyze" in result.stdout
        assert "deduplicate" in result.stdout
        assert "idiomatize" in result.stdout
        assert "pattern" in result.stdout

    @pytest.mark.e2e
    def test_analyze_help(self, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "analyze", "--help"],
            capture_output=True,
            text=True,
        )

        assert result.returncode == 0
        assert "--report" in result.stdout

    @pytest.mark.e2e
    def test_deduplicate_help(self, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "deduplicate", "--help"],
            capture_output=True,
            text=True,
        )

        assert result.returncode == 0
        assert "--yes" in result.stdout
        assert "--commit" in result.stdout
        assert "--report" in result.stdout


class TestAnalyzeCommand:
    """Test analyze command."""

    @pytest.mark.e2e
    @pytest.mark.skip(reason="Requires LLM mocking - sidecar spawns Python process")
    def test_analyze_basic(self, git_project_with_duplicates: Path, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "analyze", str(git_project_with_duplicates)],
            capture_output=True,
            text=True,
            input="y\n",
        )

        assert result.returncode == 0
        assert "Analysis Results" in result.stdout
        assert "Total files" in result.stdout

    @pytest.mark.e2e
    @pytest.mark.skip(reason="Requires LLM mocking - sidecar spawns Python process")
    def test_analyze_with_report_flag(self, git_project_with_duplicates: Path, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "analyze", "--report", str(git_project_with_duplicates)],
            capture_output=True,
            text=True,
            input="y\n",
        )

        assert result.returncode == 0
        assert "Analysis Results" in result.stdout


class TestMCPMode:
    """Test MCP server mode."""

    @pytest.mark.e2e
    def test_mcp_initialize(self, tmp_path: Path, built_cli: Path):
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

            tools = response["result"]["tools"]
            assert "list_files" in tools
            assert "get_symbols" in tools
            assert "read_file" in tools
            assert "apply_diff" in tools
            assert "apply_diff_safe" in tools
            assert "git_checkpoint" in tools
            assert "git_rollback" in tools
            assert "get_complexity" in tools
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_mcp_list_files(self, git_project_with_duplicates: Path, built_cli: Path):
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        list_req = json.dumps({"jsonrpc": "2.0", "id": 2, "method": "list_files", "params": {}})

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(git_project_with_duplicates)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{list_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            list_response = json.loads(lines[1])
            assert "result" in list_response
            assert "files" in list_response["result"]

            files = list_response["result"]["files"]
            file_paths = [f["path"] for f in files]
            assert "auth.py" in file_paths
            assert "user.py" in file_paths
        finally:
            proc.terminate()
            proc.wait()


class TestReportGeneration:
    """Test report generation functionality."""

    @pytest.mark.e2e
    @pytest.mark.skip(reason="Requires LLM mocking - sidecar spawns Python process")
    def test_analyze_generates_baseline_report(self, git_project_with_duplicates: Path, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "analyze", "--report", str(git_project_with_duplicates)],
            capture_output=True,
            text=True,
            input="y\n",
        )

        assert result.returncode == 0
        assert "Analysis Results" in result.stdout

    @pytest.mark.e2e
    @pytest.mark.skip(reason="Requires LLM mocking - sidecar spawns Python process")
    def test_deduplicate_with_report_flag(self, git_project_with_duplicates: Path, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "deduplicate", "--yes", "--report", str(git_project_with_duplicates)],
            capture_output=True,
            text=True,
            timeout=60,
            input="y\n",
        )

        assert result.returncode == 0
        assert "Deduplication" in result.stdout or "Refactoring Plan" in result.stdout


class TestGitIntegration:
    """Test git integration in CLI."""

    @pytest.mark.e2e
    def test_checkpoint_via_mcp(self, git_project_with_duplicates: Path, built_cli: Path):
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        checkpoint_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "git_checkpoint",
            "params": {"message": "test checkpoint"}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(git_project_with_duplicates)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{checkpoint_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response
            assert response["result"]["success"] is True
            assert "commit_hash" in response["result"]
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_rollback_via_mcp(self, git_project_with_duplicates: Path, built_cli: Path):
        original_content = (git_project_with_duplicates / "auth.py").read_text()

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        checkpoint_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "git_checkpoint",
            "params": {"message": "before modification"}
        })
        diff_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 3,
            "method": "apply_diff",
            "params": {
                "path": "auth.py",
                "diff": "--- a/auth.py\n+++ b/auth.py\n@@ -1,4 +1,4 @@\n def validate_email(email):\n-    \"\"\"Validate email address.\"\"\"\n+    \"\"\"Validate email address - modified.\"\"\"\n"
            }
        })
        rollback_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 4,
            "method": "git_rollback",
            "params": {}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(git_project_with_duplicates)],
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

            restored_content = (git_project_with_duplicates / "auth.py").read_text()
            assert restored_content == original_content
        finally:
            proc.terminate()
            proc.wait()


class TestErrorHandling:
    """Test error handling in CLI."""

    @pytest.mark.e2e
    def test_analyze_nonexistent_path(self, built_cli: Path):
        result = subprocess.run(
            [str(built_cli), "analyze", "/nonexistent/path"],
            capture_output=True,
            text=True,
        )

        assert result.returncode != 0

    @pytest.mark.e2e
    def test_mcp_file_not_found(self, tmp_path: Path, built_cli: Path):
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        read_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "read_file",
            "params": {"path": "nonexistent.py"}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(tmp_path)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{read_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "error" in response
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_mcp_invalid_method(self, tmp_path: Path, built_cli: Path):
        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        invalid_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "nonexistent_method",
            "params": {}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(tmp_path)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{invalid_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "error" in response
            assert "Method not found" in response["error"]["message"]
        finally:
            proc.terminate()
            proc.wait()


class TestMultiLanguageSupport:
    """Test multi-language support in CLI."""

    @pytest.mark.e2e
    def test_python_symbol_extraction(self, tmp_path: Path, built_cli: Path):
        (tmp_path / "main.py").write_text('''
def hello():
    return "hello"

class World:
    pass
''')

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        symbols_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "get_symbols",
            "params": {"path": "main.py"}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(tmp_path)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{symbols_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response

            symbols = response["result"]["symbols"]
            names = [s["name"] for s in symbols]
            assert "hello" in names
            assert "World" in names
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_javascript_symbol_extraction(self, tmp_path: Path, built_cli: Path):
        (tmp_path / "app.js").write_text('''
function hello() {
    return "hello";
}

class World {
    constructor() {}
}
''')

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        symbols_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "get_symbols",
            "params": {"path": "app.js"}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(tmp_path)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{symbols_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response

            symbols = response["result"]["symbols"]
            names = [s["name"] for s in symbols]
            assert "hello" in names
            assert "World" in names
        finally:
            proc.terminate()
            proc.wait()

    @pytest.mark.e2e
    def test_go_symbol_extraction(self, tmp_path: Path, built_cli: Path):
        (tmp_path / "main.go").write_text('''package main

func hello() string {
    return "hello"
}

func main() {
    hello()
}
''')

        init_req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}})
        symbols_req = json.dumps({
            "jsonrpc": "2.0",
            "id": 2,
            "method": "get_symbols",
            "params": {"path": "main.go"}
        })

        proc = subprocess.Popen(
            [str(built_cli), "mcp", str(tmp_path)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )

        try:
            stdout, _ = proc.communicate(input=f"{init_req}\n{symbols_req}\n", timeout=10)
            lines = stdout.strip().split("\n")

            response = json.loads(lines[1])
            assert "result" in response

            symbols = response["result"]["symbols"]
            names = [s["name"] for s in symbols]
            assert "hello" in names
            assert "main" in names
        finally:
            proc.terminate()
            proc.wait()
