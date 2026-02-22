"""
MCP entry point for Python sidecar.

Architecture:
- Go runs MCP Server (provides tools: read_file, get_symbols, etc.)
- Python runs MCP Client (calls Go tools)
- Go spawns Python with command type, Python executes and returns results
"""

import argparse
import asyncio
import json
import logging
import sys
import os
from typing import Optional, Dict, Any

from ai_sidecar.mcp import MCPClient, MCPError
from ai_sidecar.agents import (
    AnalyzerAgent,
    DeduplicatorAgent,
    IdiomatizerAgent,
    PatternAgent,
    ValidatorAgent,
)
from ai_sidecar.embeddings import EmbeddingService
from ai_sidecar.llm import LLMRouter
from ai_sidecar.models import (
    AnalyzeRequest,
    AnalyzeResult,
    DeduplicateRequest,
    IdiomatizeRequest,
    PatternRequest,
    RefactorPlan,
    FileInfo,
    Language,
)

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    stream=sys.stderr,
)
logger = logging.getLogger(__name__)


class MCPSidecar:
    """
    Python sidecar that connects to Go MCP server.
    
    Communication flow:
    1. Go spawns this process with --command and --root args
    2. Python connects MCP client to Go via stdin/stdout
    3. Python executes the requested command using MCP tools
    4. Python prints result JSON to stdout (Go reads this)
    """

    def __init__(self):
        self.mcp: Optional[MCPClient] = None
        self.embedding_service: Optional[EmbeddingService] = None
        self.llm_router: Optional[LLMRouter] = None
        self.analyzer: Optional[AnalyzerAgent] = None
        self.deduplicator: Optional[DeduplicatorAgent] = None
        self.idiomatizer: Optional[IdiomatizerAgent] = None
        self.pattern_agent: Optional[PatternAgent] = None
        self.validator: Optional[ValidatorAgent] = None
        self.root_dir: str = "."
        self._plans: Dict[str, RefactorPlan] = {}

    async def initialize(self, root_dir: str):
        self.root_dir = root_dir
        
        self.mcp = MCPClient()
        await self.mcp.connect()

        init_result = await self.mcp.initialize(root_dir)
        logger.info(f"MCP initialized: {init_result}")

        self.embedding_service = EmbeddingService()
        await self.embedding_service.initialize()

        self.llm_router = LLMRouter()

        self.analyzer = AnalyzerAgent(llm_router=self.llm_router, mcp_client=self.mcp)
        self.deduplicator = DeduplicatorAgent(
            self.embedding_service, llm_router=self.llm_router, mcp_client=self.mcp
        )
        self.idiomatizer = IdiomatizerAgent(llm_router=self.llm_router, mcp_client=self.mcp)
        self.pattern_agent = PatternAgent(llm_router=self.llm_router, mcp_client=self.mcp)
        self.validator = ValidatorAgent(mcp_client=self.mcp)
        self.validator.set_agents(self.deduplicator, self.idiomatizer, self.pattern_agent)

        logger.info("MCPSidecar initialized successfully")

    async def run_command(self, command: str, path: str) -> Dict[str, Any]:
        """Execute a single command and return result."""
        logger.info(f"Running command: {command} on {path}")
        
        try:
            if command == "analyze":
                return await self._analyze(path)
            elif command == "deduplicate":
                return await self._deduplicate(path)
            elif command == "idiomatize":
                return await self._idiomatize(path)
            elif command == "pattern":
                return await self._pattern(path)
            elif command == "apply_plan":
                return await self._apply_plan(path)
            else:
                return {"error": f"Unknown command: {command}"}
        except Exception as e:
            logger.exception(f"Command failed: {e}")
            return {"error": str(e)}

    async def _analyze(self, path: str) -> Dict[str, Any]:
        files_data = await self.mcp.list_files()
        files = [
            FileInfo(path=f["path"], content="", hash=f.get("hash"))
            for f in files_data.get("files", [])
        ]

        request = AnalyzeRequest(path=path, files=files)
        result = await self.analyzer.analyze(request)
        return result.model_dump()

    async def _deduplicate(self, path: str) -> Dict[str, Any]:
        files_data = await self.mcp.list_files()
        files = [
            FileInfo(path=f["path"], content="", hash=f.get("hash"))
            for f in files_data.get("files", [])
        ]

        request = DeduplicateRequest(path=path, files=files, similarity_threshold=0.85)
        plan = await self.deduplicator.find_duplicates(request)
        self._plans[plan.session_id] = plan
        return plan.model_dump()

    async def _idiomatize(self, path: str) -> Dict[str, Any]:
        files_data = await self.mcp.list_files()
        files = [
            FileInfo(path=f["path"], content="", hash=f.get("hash"))
            for f in files_data.get("files", [])
        ]

        request = IdiomatizeRequest(path=path, files=files, language=Language.PYTHON)
        plan = await self.idiomatizer.idiomatize(request)
        self._plans[plan.session_id] = plan
        return plan.model_dump()

    async def _pattern(self, path: str) -> Dict[str, Any]:
        files_data = await self.mcp.list_files()
        files = [
            FileInfo(path=f["path"], content="", hash=f.get("hash"))
            for f in files_data.get("files", [])
        ]

        request = PatternRequest(pattern="", path=path, files=files)
        plan = await self.pattern_agent.apply_pattern(request)
        self._plans[plan.session_id] = plan
        return plan.model_dump()

    async def _apply_plan(self, path: str) -> Dict[str, Any]:
        """
        Apply a stored refactoring plan with safety checks.
        
        This command applies all changes from a previously generated plan.
        Each change is applied with:
        1. Automatic checkpoint before diff
        2. Test execution after diff
        3. Automatic rollback if tests fail
        
        Note: Requires session_id in the request. Use the session_id from
        a previous deduplicate/idiomatize/pattern command.
        """
        return {
            "error": "apply_plan requires session_id parameter. "
                     "First run deduplicate/idiomatize/pattern to get a session_id, "
                     "then call apply_plan with that session_id."
        }
    
    async def apply_plan_by_session(self, session_id: str, run_tests: bool = True) -> Dict[str, Any]:
        """
        Apply a stored refactoring plan by session ID.
        
        Args:
            session_id: The session ID from a previous plan generation
            run_tests: Whether to run tests after each diff (default: True)
            
        Returns:
            Dict with applied changes and any errors
        """
        plan = self._plans.get(session_id)
        if not plan:
            return {"error": f"No plan found with session_id: {session_id}"}
        
        if not plan.changes:
            return {
                "session_id": session_id,
                "success": True,
                "applied_count": 0,
                "message": "No changes to apply"
            }
        
        results = []
        applied_count = 0
        any_rolled_back = False
        
        for i, change in enumerate(plan.changes):
            if not change.original and change.modified:
                result = await self.mcp.apply_diff_safe(
                    path=change.path,
                    diff=self._create_new_file_diff(change.path, change.modified),
                    run_tests=run_tests,
                )
            elif change.original and change.modified:
                result = await self.mcp.apply_diff_safe(
                    path=change.path,
                    diff=self._create_modify_diff(change.original, change.modified),
                    run_tests=run_tests,
                )
            else:
                result = {"success": False, "error": "Invalid change: no original or modified content"}
            
            results.append({
                "path": change.path,
                "description": change.description,
                "result": result,
            })
            
            if result.get("success"):
                applied_count += 1
            elif result.get("rolled_back"):
                any_rolled_back = True
                logger.warning(f"Change {i+1} to {change.path} was rolled back due to test failure")
                break
            else:
                logger.error(f"Change {i+1} to {change.path} failed: {result.get('error')}")
                break
        
        return {
            "session_id": session_id,
            "success": applied_count == len(plan.changes) and not any_rolled_back,
            "applied_count": applied_count,
            "total_changes": len(plan.changes),
            "any_rolled_back": any_rolled_back,
            "results": results,
        }
    
    def _create_new_file_diff(self, path: str, content: str) -> str:
        """Create a diff for creating a new file."""
        return f"--- /dev/null\n+++ b/{path}\n@@ -0,0 +1,{len(content.splitlines())} @@\n" + \
               "\n".join(f"+{line}" for line in content.splitlines())
    
    def _create_modify_diff(self, original: str, modified: str) -> str:
        """Create a simple diff for modifying a file."""
        orig_lines = original.splitlines()
        mod_lines = modified.splitlines()
        
        diff_lines = [f"--- a/file", f"+++ b/file"]
        diff_lines.append(f"@@ -1,{len(orig_lines)} +1,{len(mod_lines)} @@")
        
        for line in orig_lines:
            diff_lines.append(f"-{line}")
        for line in mod_lines:
            diff_lines.append(f"+{line}")
        
        return "\n".join(diff_lines)

    async def shutdown(self):
        logger.info("Shutting down MCPSidecar...")

        if self.embedding_service:
            await self.embedding_service.shutdown()

        if self.mcp:
            await self.mcp.shutdown()

        logger.info("Shutdown complete")


async def main():
    parser = argparse.ArgumentParser(description="reducto AI Sidecar (MCP mode)")
    parser.add_argument("--root", default=".", help="Root directory")
    parser.add_argument("--command", default="analyze", help="Command to execute")
    parser.add_argument("--session-id", default=None, help="Session ID for apply_plan command")
    parser.add_argument("--run-tests", action="store_true", default=True, help="Run tests after applying changes")
    args = parser.parse_args()

    sidecar = MCPSidecar()
    result = None
    exit_code = 0
    
    try:
        await sidecar.initialize(args.root)
        
        if args.command == "apply_plan" and args.session_id:
            result = await sidecar.apply_plan_by_session(args.session_id, run_tests=args.run_tests)
        else:
            result = await sidecar.run_command(args.command, args.root)
        
    except MCPError as e:
        logger.error(f"MCP error: {e}")
        result = {"error": str(e)}
        exit_code = 1
    except Exception as e:
        logger.exception(f"Fatal error: {e}")
        result = {"error": str(e)}
        exit_code = 1
    finally:
        await sidecar.shutdown()

    if result:
        print("RESULT:" + json.dumps({"status": "success" if "error" not in result else "error", "data": result}), file=sys.stderr)
    
    sys.exit(exit_code)


if __name__ == "__main__":
    asyncio.run(main())
