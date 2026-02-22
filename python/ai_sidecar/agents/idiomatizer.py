"""
Idiomatizer agent for transforming code to idiomatic patterns.
"""

import uuid
from typing import List, Dict, Optional

from ai_sidecar.models import (
    IdiomatizeRequest,
    RefactorPlan,
    FileChange,
    Language,
    ModelTier,
)


class IdiomatizerAgent:
    def __init__(self, llm_router=None, mcp_client=None):
        self.llm = llm_router
        self.mcp = mcp_client
        self._session_plans: Dict[str, RefactorPlan] = {}

    async def idiomatize(self, request: IdiomatizeRequest) -> RefactorPlan:
        path = request.path
        files = request.files
        language = request.language

        changes = []
        for file in files:
            file_changes = await self._idiomatize_file(file, language)
            changes.extend(file_changes)

        session_id = str(uuid.uuid4())
        plan = RefactorPlan(
            session_id=session_id,
            changes=changes,
            description=f"Found {len(changes)} opportunities for idiomatic improvements.",
        )

        self._session_plans[session_id] = plan
        return plan

    async def _idiomatize_file(
        self,
        file,
        language: Language,
    ) -> List[FileChange]:
        changes = []
        if hasattr(file, 'content'):
            content = file.content
            path = file.path
        else:
            content = file["content"]
            path = file["path"]

        if language == Language.PYTHON:
            changes = self._idiomatize_python(content, path)
        elif language in (Language.JAVASCRIPT, Language.TYPESCRIPT):
            changes = self._idiomatize_js(content, path, language)
        elif language == Language.GO:
            changes = self._idiomatize_go(content, path)

        return changes

    def _idiomatize_python(self, content: str, path: str) -> List[FileChange]:
        changes = []
        lines = content.split("\n")

        for i, line in enumerate(lines):
            result = self._try_python_idiom(line, lines, i)
            if result:
                original, modified, description = result
                changes.append(FileChange(
                    path=path,
                    original=original,
                    modified=modified,
                    description=f"Line {i+1}: {description}",
                ))

        return changes

    def _try_python_idiom(self, line: str, lines: List[str], idx: int) -> Optional[tuple]:
        stripped = line.strip()

        if self._is_for_append_pattern(lines, idx):
            return self._convert_to_list_comp(lines, idx)

        if self._is_dict_init_pattern(stripped):
            return self._suggest_dict_comp(stripped)

        if self._is_string_concat_pattern(lines, idx):
            return self._suggest_fstring(lines, idx)

        return None

    def _is_for_append_pattern(self, lines: List[str], idx: int) -> bool:
        if idx + 1 >= len(lines):
            return False

        line = lines[idx].strip()
        if not line.startswith("for "):
            return False

        next_line = lines[idx + 1].strip()
        if ".append(" in next_line:
            return True

        return False

    def _convert_to_list_comp(self, lines: List[str], idx: int) -> Optional[tuple]:
        for_line = lines[idx]
        append_line = lines[idx + 1].strip() if idx + 1 < len(lines) else ""

        import re
        for_match = re.match(r"for\s+(\w+)\s+in\s+(.+?):", for_line.strip())
        if not for_match:
            return None

        var = for_match.group(1)
        iterable = for_match.group(2)

        append_match = re.match(r"(\w+)\.append\((.+?)\)", append_line)
        if not append_match:
            return None

        list_var = append_match.group(1)
        expr = append_match.group(2)

        indent = len(for_line) - len(for_line.lstrip())
        list_comp = f"{' ' * indent}{list_var} = [{expr} for {var} in {iterable}]"

        original = f"{for_line}\n{append_line}"
        return (original, list_comp, "Convert for-loop with append to list comprehension")

    def _is_dict_init_pattern(self, line: str) -> bool:
        return "= {}" in line and "if " not in line

    def _suggest_dict_comp(self, line: str) -> Optional[tuple]:
        return None

    def _is_string_concat_pattern(self, lines: List[str], idx: int) -> bool:
        line = lines[idx]
        return '"' in line or "'" in line and "+" in line

    def _suggest_fstring(self, lines: List[str], idx: int) -> Optional[tuple]:
        return None

    def _idiomatize_js(
        self,
        content: str,
        path: str,
        language: Language,
    ) -> List[FileChange]:
        changes = []

        if "var " in content:
            changes.append(FileChange(
                path=path,
                original="var ",
                modified="const ",
                description="Replace 'var' with 'const' or 'let'",
            ))

        if "for (var " in content or "for (let " in content:
            changes.append(FileChange(
                path=path,
                original="for (",
                modified="for (const ",
                description="Consider using for...of or array methods",
            ))

        return changes

    def _idiomatize_go(self, content: str, path: str) -> List[FileChange]:
        changes = []

        if "if err != nil {" in content:
            pass

        return changes

    def get_plan(self, session_id: str) -> Optional[RefactorPlan]:
        return self._session_plans.get(session_id)

    async def suggest_idiomatic_version(
        self,
        code: str,
        language: Language,
        tier: ModelTier = ModelTier.MEDIUM,
    ) -> str:
        """Use LLM to suggest idiomatic version of code."""
        if not self.llm:
            return "LLM not available for idiomatization"

        goal = f"Transform this code to be more idiomatic {language.value}"
        return await self.llm.suggest_refactor(code, language.value, goal, tier=tier)
