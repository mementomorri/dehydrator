"""
LSP (Language Server Protocol) integration module.
"""

from typing import Optional, Dict, List, Any
import subprocess
import json


class LSPClient:
    """Base class for LSP client implementations."""

    def __init__(self, project_root: str):
        self.project_root = project_root
        self.process: Optional[subprocess.Popen] = None

    async def initialize(self) -> bool:
        raise NotImplementedError

    async def shutdown(self) -> None:
        if self.process:
            self.process.terminate()
            self.process = None

    async def get_definition(self, file_path: str, line: int, column: int) -> Optional[Dict]:
        raise NotImplementedError

    async def get_references(self, file_path: str, line: int, column: int) -> List[Dict]:
        raise NotImplementedError

    async def get_completions(self, file_path: str, line: int, column: int) -> List[Dict]:
        raise NotImplementedError


class PythonLSPClient(LSPClient):
    """LSP client for Python (pyright/pylsp)."""

    def __init__(self, project_root: str):
        super().__init__(project_root)
        self.server_type = "pyright"

    async def initialize(self) -> bool:
        try:
            if self._check_pyright():
                self.server_type = "pyright"
                return True
            elif self._check_pylsp():
                self.server_type = "pylsp"
                return True
        except Exception:
            pass
        return False

    def _check_pyright(self) -> bool:
        try:
            result = subprocess.run(
                ["pyright", "--version"],
                capture_output=True,
                timeout=5,
            )
            return result.returncode == 0
        except (subprocess.TimeoutExpired, FileNotFoundError):
            return False

    def _check_pylsp(self) -> bool:
        try:
            result = subprocess.run(
                ["pylsp", "--version"],
                capture_output=True,
                timeout=5,
            )
            return result.returncode == 0
        except (subprocess.TimeoutExpired, FileNotFoundError):
            return False

    async def get_definition(self, file_path: str, line: int, column: int) -> Optional[Dict]:
        return None

    async def get_references(self, file_path: str, line: int, column: int) -> List[Dict]:
        return []


class GoLSPClient(LSPClient):
    """LSP client for Go (gopls)."""

    async def initialize(self) -> bool:
        try:
            result = subprocess.run(
                ["gopls", "version"],
                capture_output=True,
                timeout=5,
            )
            return result.returncode == 0
        except (subprocess.TimeoutExpired, FileNotFoundError):
            return False

    async def get_definition(self, file_path: str, line: int, column: int) -> Optional[Dict]:
        return None

    async def get_references(self, file_path: str, line: int, column: int) -> List[Dict]:
        return []


class TypeScriptLSPClient(LSPClient):
    """LSP client for TypeScript (typescript-language-server)."""

    async def initialize(self) -> bool:
        try:
            result = subprocess.run(
                ["typescript-language-server", "--version"],
                capture_output=True,
                timeout=5,
            )
            return result.returncode == 0
        except (subprocess.TimeoutExpired, FileNotFoundError):
            return False

    async def get_definition(self, file_path: str, line: int, column: int) -> Optional[Dict]:
        return None

    async def get_references(self, file_path: str, line: int, column: int) -> List[Dict]:
        return []


def create_lsp_client(language: str, project_root: str) -> Optional[LSPClient]:
    """Factory function to create appropriate LSP client."""
    clients = {
        "python": PythonLSPClient,
        "go": GoLSPClient,
        "typescript": TypeScriptLSPClient,
        "javascript": TypeScriptLSPClient,
    }

    client_class = clients.get(language)
    if client_class:
        return client_class(project_root)
    return None
