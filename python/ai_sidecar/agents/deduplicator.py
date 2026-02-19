"""
Deduplicator agent for finding and eliminating code duplication.
"""

import uuid
from typing import List, Dict, Optional

from ai_sidecar.models import (
    DeduplicateRequest,
    RefactorPlan,
    FileChange,
    DuplicateGroup,
    CodeBlock,
    Language,
    ComplexityMetrics,
)
from ai_sidecar.embeddings import EmbeddingService


class DeduplicatorAgent:
    def __init__(self, embedding_service: EmbeddingService):
        self.embedding_service = embedding_service
        self._session_plans: Dict[str, RefactorPlan] = {}

    async def find_duplicates(self, request: DeduplicateRequest) -> RefactorPlan:
        path = request.path
        files = request.files
        threshold = request.similarity_threshold

        blocks = await self._extract_blocks(files)
        duplicate_groups = await self.embedding_service.find_duplicates(blocks, threshold)

        changes = []
        for group in duplicate_groups:
            if len(group) >= 2:
                change = await self._create_dedup_change(group, path)
                if change:
                    changes.append(change)

        session_id = str(uuid.uuid4())
        plan = RefactorPlan(
            session_id=session_id,
            changes=changes,
            description=f"Found {len(duplicate_groups)} duplicate code blocks. "
                       f"Proposed {len(changes)} refactoring changes.",
        )

        self._session_plans[session_id] = plan
        return plan

    async def _extract_blocks(self, files: List[Dict]) -> List[CodeBlock]:
        blocks = []

        for file in files:
            path = file["path"]
            content = file["content"]
            language = self._detect_language(path)

            file_blocks = await self._parse_blocks(content, path, language)
            blocks.extend(file_blocks)

        return blocks

    def _detect_language(self, path: str) -> Language:
        if path.endswith(".py"):
            return Language.PYTHON
        elif path.endswith(".js"):
            return Language.JAVASCRIPT
        elif path.endswith((".ts", ".tsx")):
            return Language.TYPESCRIPT
        elif path.endswith(".go"):
            return Language.GO
        return Language.UNKNOWN

    async def _parse_blocks(
        self,
        content: str,
        path: str,
        language: Language,
    ) -> List[CodeBlock]:
        blocks = []
        lines = content.split("\n")

        if language == Language.PYTHON:
            blocks = self._parse_python_blocks(lines, path, language)
        elif language in (Language.JAVASCRIPT, Language.TYPESCRIPT):
            blocks = self._parse_js_blocks(lines, path, language)
        elif language == Language.GO:
            blocks = self._parse_go_blocks(lines, path, language)

        return blocks

    def _parse_python_blocks(
        self,
        lines: List[str],
        path: str,
        language: Language,
    ) -> List[CodeBlock]:
        blocks = []

        i = 0
        while i < len(lines):
            line = lines[i]
            stripped = line.strip()

            if stripped.startswith(("def ", "async def ")):
                start = i
                name = self._extract_function_name(stripped)
                end = self._find_block_end(lines, i)
                content = "\n".join(lines[start:end])

                blocks.append(CodeBlock(
                    id=f"{path}:{start}:{name}",
                    file=path,
                    start_line=start + 1,
                    end_line=end,
                    content=content,
                    language=language,
                    symbol_type="function",
                    symbol_name=name,
                    metrics=self._calculate_metrics(content),
                ))
                i = end

            i += 1

        return blocks

    def _parse_js_blocks(
        self,
        lines: List[str],
        path: str,
        language: Language,
    ) -> List[CodeBlock]:
        blocks = []

        i = 0
        while i < len(lines):
            line = lines[i]
            stripped = line.strip()

            if "function " in stripped:
                start = i
                name = self._extract_js_function_name(stripped)
                end = self._find_js_block_end(lines, i)
                content = "\n".join(lines[start:end])

                blocks.append(CodeBlock(
                    id=f"{path}:{start}:{name}",
                    file=path,
                    start_line=start + 1,
                    end_line=end,
                    content=content,
                    language=language,
                    symbol_type="function",
                    symbol_name=name or "anonymous",
                    metrics=self._calculate_metrics(content),
                ))
                i = end

            i += 1

        return blocks

    def _parse_go_blocks(
        self,
        lines: List[str],
        path: str,
        language: Language,
    ) -> List[CodeBlock]:
        blocks = []

        i = 0
        while i < len(lines):
            line = lines[i]
            stripped = line.strip()

            if stripped.startswith("func "):
                start = i
                name = self._extract_go_function_name(stripped[5:])
                end = self._find_js_block_end(lines, i)
                content = "\n".join(lines[start:end])

                blocks.append(CodeBlock(
                    id=f"{path}:{start}:{name}",
                    file=path,
                    start_line=start + 1,
                    end_line=end,
                    content=content,
                    language=language,
                    symbol_type="function",
                    symbol_name=name,
                    metrics=self._calculate_metrics(content),
                ))
                i = end

            i += 1

        return blocks

    def _extract_function_name(self, line: str) -> str:
        import re
        match = re.match(r"(?:async\s+)?def\s+(\w+)", line)
        if match:
            return match.group(1)
        return "unknown"

    def _extract_js_function_name(self, line: str) -> Optional[str]:
        import re
        patterns = [
            r"function\s+(\w+)",
            r"(\w+)\s*=\s*(?:async\s*)?function",
            r"(\w+)\s*:\s*(?:async\s*)?function",
            r"const\s+(\w+)\s*=\s*(?:async\s*)?\(",
        ]

        for pattern in patterns:
            match = re.search(pattern, line)
            if match:
                return match.group(1)
        return None

    def _extract_go_function_name(self, decl: str) -> str:
        if decl.startswith("("):
            paren_end = decl.find(")")
            if paren_end > 0:
                return decl[paren_end + 1:].split("(")[0].strip()
        return decl.split("(")[0].strip()

    def _find_block_end(self, lines: List[str], start: int) -> int:
        indent = len(lines[start]) - len(lines[start].lstrip())
        for i in range(start + 1, len(lines)):
            if lines[i].strip() and not lines[i].startswith(" " * (indent + 1)):
                return i
        return len(lines)

    def _find_js_block_end(self, lines: List[str], start: int) -> int:
        brace_count = 0
        for i in range(start, len(lines)):
            brace_count += lines[i].count("{") - lines[i].count("}")
            if brace_count == 0 and i > start:
                return i + 1
        return len(lines)

    def _calculate_metrics(self, content: str) -> ComplexityMetrics:
        lines = [l for l in content.split("\n") if l.strip()]
        loc = len(lines)

        cyclomatic = 1
        cognitive = 0

        for line in lines:
            stripped = line.strip()
            if any(stripped.startswith(kw) for kw in ["if ", "for ", "while ", "case "]):
                cyclomatic += 1
                cognitive += 1
            if " and " in stripped or " or " in stripped:
                cyclomatic += 1

        return ComplexityMetrics(
            cyclomatic_complexity=cyclomatic,
            cognitive_complexity=cognitive,
            lines_of_code=loc,
        )

    async def _create_dedup_change(
        self,
        group: List[CodeBlock],
        base_path: str,
    ) -> Optional[FileChange]:
        if len(group) < 2:
            return None

        primary = group[0]
        combined_names = "_".join(b.symbol_name for b in group[:3])
        if len(combined_names) > 30:
            combined_names = combined_names[:30]

        new_util_path = f"utils/{primary.symbol_name}_dedup.py"
        new_content = self._generate_util_function(primary)

        description = (
            f"Extract duplicate '{primary.symbol_name}' found in {len(group)} files: "
            f"{', '.join(b.file for b in group[:3])}"
        )

        return FileChange(
            path=new_util_path,
            original="",
            modified=new_content,
            description=description,
        )

    def _generate_util_function(self, block: CodeBlock) -> str:
        lines = block.content.split("\n")
        dedented = []
        min_indent = float("inf")

        for line in lines[1:]:
            if line.strip():
                indent = len(line) - len(line.lstrip())
                min_indent = min(min_indent, indent)

        for line in lines:
            if len(line) >= min_indent and line.strip():
                dedented.append(line[min_indent:])
            else:
                dedented.append(line)

        return "\n".join(dedented)

    def get_plan(self, session_id: str) -> Optional[RefactorPlan]:
        return self._session_plans.get(session_id)
