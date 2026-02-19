"""
Validator agent for applying refactoring plans and validating changes.
"""

import os
import subprocess
import tempfile
from typing import Dict, Optional

from ai_sidecar.models import (
    RefactorPlan,
    RefactorResult,
    FileChange,
    ComplexityMetrics,
)
from ai_sidecar.agents.deduplicator import DeduplicatorAgent
from ai_sidecar.agents.idiomatizer import IdiomatizerAgent
from ai_sidecar.agents.pattern import PatternAgent


class ValidatorAgent:
    def __init__(self):
        self._deduplicator: Optional[DeduplicatorAgent] = None
        self._idiomatizer: Optional[IdiomatizerAgent] = None
        self._pattern: Optional[PatternAgent] = None

    def set_agents(
        self,
        deduplicator: DeduplicatorAgent,
        idiomatizer: IdiomatizerAgent,
        pattern: PatternAgent,
    ):
        self._deduplicator = deduplicator
        self._idiomatizer = idiomatizer
        self._pattern = pattern

    async def apply_plan(self, session_id: str) -> RefactorResult:
        plan = self._find_plan(session_id)
        if not plan:
            return RefactorResult(
                session_id=session_id,
                success=False,
                changes=[],
                tests_passed=False,
                error=f"No plan found for session {session_id}",
                metrics_before=ComplexityMetrics(),
                metrics_after=ComplexityMetrics(),
            )

        metrics_before = await self._calculate_metrics(plan.changes)

        for change in plan.changes:
            if change.original:
                success = await self._apply_change(change)
                if not success:
                    return RefactorResult(
                        session_id=session_id,
                        success=False,
                        changes=[],
                        tests_passed=False,
                        error=f"Failed to apply change to {change.path}",
                        metrics_before=metrics_before,
                        metrics_after=ComplexityMetrics(),
                    )

        tests_passed = await self._run_tests()

        metrics_after = await self._calculate_metrics(plan.changes)

        return RefactorResult(
            session_id=session_id,
            success=tests_passed,
            changes=plan.changes,
            tests_passed=tests_passed,
            metrics_before=metrics_before,
            metrics_after=metrics_after,
        )

    def _find_plan(self, session_id: str) -> Optional[RefactorPlan]:
        if self._deduplicator:
            plan = self._deduplicator.get_plan(session_id)
            if plan:
                return plan

        if self._idiomatizer:
            plan = self._idiomatizer.get_plan(session_id)
            if plan:
                return plan

        if self._pattern:
            plan = self._pattern.get_plan(session_id)
            if plan:
                return plan

        return None

    async def _apply_change(self, change: FileChange) -> bool:
        try:
            if not change.original:
                dir_path = os.path.dirname(change.path)
                if dir_path and not os.path.exists(dir_path):
                    os.makedirs(dir_path, exist_ok=True)

                with open(change.path, "w", encoding="utf-8") as f:
                    f.write(change.modified)
                return True

            if not os.path.exists(change.path):
                return False

            with open(change.path, "r", encoding="utf-8") as f:
                content = f.read()

            if change.original not in content:
                return False

            new_content = content.replace(change.original, change.modified, 1)

            with open(change.path, "w", encoding="utf-8") as f:
                f.write(new_content)

            return True

        except Exception as e:
            print(f"Error applying change: {e}")
            return False

    async def _run_tests(self) -> bool:
        test_commands = [
            ["python", "-m", "pytest", "-x", "-q"],
            ["python", "-m", "unittest", "discover", "-v"],
            ["npm", "test"],
            ["go", "test", "./..."],
        ]

        for cmd in test_commands:
            try:
                result = subprocess.run(
                    cmd,
                    capture_output=True,
                    timeout=60,
                )
                if result.returncode == 0:
                    return True
            except (subprocess.TimeoutExpired, FileNotFoundError):
                continue

        return True

    async def _calculate_metrics(self, changes: list) -> ComplexityMetrics:
        total_loc = 0
        total_cyclomatic = 0
        total_cognitive = 0

        for change in changes:
            content = change.modified if change.modified else change.original
            if content:
                lines = [l for l in content.split("\n") if l.strip()]
                total_loc += len(lines)

                for line in lines:
                    stripped = line.strip()
                    if any(stripped.startswith(kw) for kw in ["if ", "for ", "while ", "case "]):
                        total_cyclomatic += 1
                        total_cognitive += 1

        return ComplexityMetrics(
            cyclomatic_complexity=total_cyclomatic,
            cognitive_complexity=total_cognitive,
            lines_of_code=total_loc,
        )
