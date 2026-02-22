"""MCP client implementation for JSON-RPC over STDIO."""

import asyncio
import json
import logging
import sys
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)


class MCPError(Exception):
    """MCP/JSON-RPC error."""
    def __init__(self, code: int, message: str, data: Any = None):
        self.code = code
        self.message = message
        self.data = data
        super().__init__(f"MCP Error {code}: {message}")


class MCPClient:
    """Client for communicating with Go MCP server over STDIO."""

    def __init__(self):
        self._request_id = 0
        self._pending: Dict[int, asyncio.Future] = {}
        self._reader: Optional[asyncio.StreamReader] = None
        self._writer: Optional[asyncio.StreamWriter] = None
        self._read_task: Optional[asyncio.Task] = None
        self._initialized = False

    async def connect(self):
        """Connect to the MCP server via STDIO."""
        loop = asyncio.get_event_loop()
        self._reader = asyncio.StreamReader()
        
        reader_protocol = asyncio.StreamReaderProtocol(self._reader)
        await loop.connect_read_pipe(lambda: reader_protocol, sys.stdin)

        writer_transport, writer_protocol = await loop.connect_write_pipe(
            asyncio.streams.FlowControlMixin,
            sys.stdout.buffer
        )
        self._writer = asyncio.StreamWriter(
            writer_transport, writer_protocol, None, loop
        )
        
        self._read_task = asyncio.create_task(self._read_responses())
        logger.info("MCP client connected via STDIO")

    async def _read_responses(self):
        """Background task to read responses from server."""
        while True:
            try:
                line = await self._reader.readline()
                if not line:
                    break
                
                response = json.loads(line.decode('utf-8').strip())
                self._handle_response(response)
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Error reading response: {e}")
                break

    def _handle_response(self, response: Dict[str, Any]):
        """Handle an incoming response."""
        request_id = response.get("id")
        if request_id is None:
            return

        future = self._pending.pop(request_id, None)
        if future is None:
            logger.warning(f"Received response for unknown request: {request_id}")
            return

        if "error" in response:
            error = response["error"]
            future.set_exception(MCPError(
                code=error.get("code", -32603),
                message=error.get("message", "Unknown error"),
                data=error.get("data"),
            ))
        else:
            future.set_result(response.get("result"))

    async def call(self, method: str, params: Optional[Dict[str, Any]] = None) -> Any:
        """Make an MCP call."""
        if self._writer is None:
            raise RuntimeError("Not connected to MCP server")

        self._request_id += 1
        request_id = self._request_id

        request = {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": method,
            "params": params or {},
        }

        future: asyncio.Future = asyncio.get_event_loop().create_future()
        self._pending[request_id] = future

        line = json.dumps(request) + "\n"
        self._writer.write(line.encode('utf-8'))
        await self._writer.drain()

        return await future

    async def initialize(self, root_dir: str) -> Dict[str, Any]:
        """Initialize the MCP connection."""
        result = await self.call("initialize", {"root_dir": root_dir})
        self._initialized = True
        logger.info(f"MCP initialized: {result}")
        return result

    async def shutdown(self):
        """Shutdown the MCP connection."""
        if self._read_task:
            self._read_task.cancel()
            try:
                await self._read_task
            except asyncio.CancelledError:
                pass
        if self._writer:
            self._writer.close()
            await self._writer.wait_closed()

    async def read_file(self, path: str) -> Dict[str, Any]:
        """Read a file from the repository."""
        return await self.call("read_file", {"path": path})

    async def get_symbols(self, path: str, content: Optional[str] = None) -> Dict[str, Any]:
        """Get symbols from a file."""
        params = {"path": path}
        if content is not None:
            params["content"] = content
        return await self.call("get_symbols", params)

    async def get_ast(self, path: str, content: Optional[str] = None) -> Dict[str, Any]:
        """Get AST for a file."""
        params = {"path": path}
        if content is not None:
            params["content"] = content
        return await self.call("get_ast", params)

    async def find_references(self, path: str, line: int, column: int) -> List[Dict[str, Any]]:
        """Find references to a symbol."""
        result = await self.call("find_references", {
            "path": path,
            "line": line,
            "column": column,
        })
        return result.get("references", [])

    async def apply_diff(self, path: str, diff: str, session_id: Optional[str] = None) -> Dict[str, Any]:
        """Apply a unified diff to a file."""
        params = {"path": path, "diff": diff}
        if session_id:
            params["session_id"] = session_id
        return await self.call("apply_diff", params)

    async def apply_diff_safe(
        self,
        path: str,
        diff: str,
        session_id: Optional[str] = None,
        run_tests: bool = True,
    ) -> Dict[str, Any]:
        """
        Apply a unified diff with automatic rollback on test failure.
        
        Flow:
        1. Create git checkpoint
        2. Apply diff
        3. Run tests (if run_tests=True)
        4. If tests fail, automatically rollback
        5. Return result with test status
        
        Returns:
            Dict with keys:
            - success: bool - whether the operation succeeded
            - path: str - the file path
            - checkpoint: str - the checkpoint commit hash
            - tests_run: bool - whether tests were run
            - tests_passed: bool - whether tests passed
            - rolled_back: bool - whether a rollback occurred
            - error: str (optional) - error message if failed
            - test_output: str (optional) - test output
        """
        params = {
            "path": path,
            "diff": diff,
            "run_tests": run_tests,
        }
        if session_id:
            params["session_id"] = session_id
        return await self.call("apply_diff_safe", params)

    async def run_tests(self) -> Dict[str, Any]:
        """Run the test suite."""
        return await self.call("run_tests")

    async def git_checkpoint(self, message: str = "checkpoint before refactoring") -> Dict[str, Any]:
        """Create a git checkpoint."""
        return await self.call("git_checkpoint", {"message": message})

    async def git_rollback(self) -> Dict[str, Any]:
        """Rollback to the last checkpoint."""
        return await self.call("git_rollback")

    async def list_files(
        self,
        include_patterns: Optional[List[str]] = None,
        exclude_patterns: Optional[List[str]] = None,
    ) -> Dict[str, Any]:
        """List files in the repository."""
        params = {}
        if include_patterns:
            params["include_patterns"] = include_patterns
        if exclude_patterns:
            params["exclude_patterns"] = exclude_patterns
        return await self.call("list_files", params)

    async def get_complexity(self, path: str, content: Optional[str] = None) -> Dict[str, Any]:
        """Get complexity metrics for a file."""
        params = {"path": path}
        if content is not None:
            params["content"] = content
        return await self.call("get_complexity", params)

    @property
    def is_initialized(self) -> bool:
        return self._initialized
