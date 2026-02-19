"""
Analyzer agent for repository analysis and complexity detection.
"""

import os
import uuid
from typing import List, Dict, Any, Optional

from ai_sidecar.models import (
    AnalyzeRequest,
    AnalyzeResult,
    ComplexityHotspot,
    Symbol,
    Language,
    ComplexityMetrics,
)


class AnalyzerAgent:
    def __init__(self):
        self._session_cache: Dict[str, Any] = {}

    async def analyze(self, request: AnalyzeRequest) -> AnalyzeResult:
        path = request.path
        files = request.files

        if not files:
            files = await self._scan_directory(path)

        symbols = await self._extract_symbols(files)
        hotspots = await self._find_hotspots(files, symbols)

        return AnalyzeResult(
            total_files=len(files),
            total_symbols=len(symbols),
            hotspots=hotspots,
            duplicates=[],
            symbols=symbols,
        )

    async def _scan_directory(self, path: str):
        files = []
        exclude_dirs = {".git", "node_modules", "venv", "__pycache__", "vendor", "dist", "build"}

        for root, dirs, filenames in os.walk(path):
            dirs[:] = [d for d in dirs if d not in exclude_dirs]

            for filename in filenames:
                filepath = os.path.join(root, filename)
                relpath = os.path.relpath(filepath, path)

                if self._should_include(filename):
                    try:
                        with open(filepath, "r", encoding="utf-8") as f:
                            content = f.read()
                        files.append({
                            "path": relpath,
                            "content": content,
                        })
                    except (UnicodeDecodeError, IOError):
                        continue

        return files

    def _should_include(self, filename: str) -> bool:
        include_exts = {".py", ".js", ".ts", ".tsx", ".go", ".java"}
        _, ext = os.path.splitext(filename)
        return ext.lower() in include_exts

    async def _extract_symbols(self, files: List[Dict]) -> List[Symbol]:
        symbols = []

        for file in files:
            content = file["content"]
            path = file["path"]
            language = self._detect_language(path)

            file_symbols = await self._parse_symbols(content, path, language)
            symbols.extend(file_symbols)

        return symbols

    def _detect_language(self, path: str) -> Language:
        _, ext = os.path.splitext(path)
        mapping = {
            ".py": Language.PYTHON,
            ".js": Language.JAVASCRIPT,
            ".ts": Language.TYPESCRIPT,
            ".tsx": Language.TYPESCRIPT,
            ".go": Language.GO,
        }
        return mapping.get(ext.lower(), Language.UNKNOWN)

    async def _parse_symbols(
        self,
        content: str,
        path: str,
        language: Language,
    ) -> List[Symbol]:
        symbols = []
        lines = content.split("\n")

        if language == Language.PYTHON:
            symbols = self._parse_python_symbols(lines, path)
        elif language in (Language.JAVASCRIPT, Language.TYPESCRIPT):
            symbols = self._parse_js_symbols(lines, path)
        elif language == Language.GO:
            symbols = self._parse_go_symbols(lines, path)

        return symbols

    def _parse_python_symbols(self, lines: List[str], path: str) -> List[Symbol]:
        symbols = []
        current_class = None

        for i, line in enumerate(lines):
            stripped = line.strip()

            if stripped.startswith("def "):
                name = self._extract_function_name(stripped[4:])
                symbols.append(Symbol(
                    name=name,
                    type="method" if current_class else "function",
                    file=path,
                    start_line=i + 1,
                    end_line=self._find_function_end(lines, i),
                    signature=self._extract_signature(stripped),
                ))

            elif stripped.startswith("class "):
                name = self._extract_class_name(stripped[6:])
                current_class = name
                symbols.append(Symbol(
                    name=name,
                    type="class",
                    file=path,
                    start_line=i + 1,
                    end_line=self._find_class_end(lines, i),
                ))

            elif stripped.startswith(("def ", "async def ")):
                pass

        return symbols

    def _parse_js_symbols(self, lines: List[str], path: str) -> List[Symbol]:
        symbols = []

        for i, line in enumerate(lines):
            stripped = line.strip()

            if "function " in stripped:
                name = self._extract_js_function_name(stripped)
                if name:
                    symbols.append(Symbol(
                        name=name,
                        type="function",
                        file=path,
                        start_line=i + 1,
                        end_line=self._find_js_function_end(lines, i),
                    ))

            elif stripped.startswith("class "):
                name = self._extract_class_name(stripped[6:])
                symbols.append(Symbol(
                    name=name,
                    type="class",
                    file=path,
                    start_line=i + 1,
                    end_line=self._find_class_end(lines, i),
                ))

        return symbols

    def _parse_go_symbols(self, lines: List[str], path: str) -> List[Symbol]:
        symbols = []

        for i, line in enumerate(lines):
            stripped = line.strip()

            if stripped.startswith("func "):
                name = self._extract_go_function_name(stripped[5:])
                symbols.append(Symbol(
                    name=name,
                    type="function",
                    file=path,
                    start_line=i + 1,
                    end_line=self._find_function_end(lines, i),
                    signature=self._extract_go_signature(stripped),
                ))

            elif stripped.startswith("type ") and " struct" in stripped:
                name = stripped[5:].split(" struct")[0]
                symbols.append(Symbol(
                    name=name,
                    type="struct",
                    file=path,
                    start_line=i + 1,
                    end_line=self._find_struct_end(lines, i),
                ))

        return symbols

    def _extract_function_name(self, decl: str) -> str:
        paren_idx = decl.find("(")
        if paren_idx > 0:
            return decl[:paren_idx].strip()
        return decl.split()[0] if decl else "unknown"

    def _extract_class_name(self, decl: str) -> str:
        for char in "(:[{":
            idx = decl.find(char)
            if idx > 0:
                return decl[:idx].strip()
        return decl.strip()

    def _extract_signature(self, line: str) -> str:
        paren_idx = line.find("(")
        if paren_idx >= 0:
            end_idx = line.rfind(")")
            if end_idx > paren_idx:
                return line[paren_idx:end_idx + 1]
        return ""

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

    def _extract_go_signature(self, line: str) -> str:
        import re
        match = re.search(r"\([^)]*\)", line)
        if match:
            return match.group(0)
        return ""

    def _find_function_end(self, lines: List[str], start: int) -> int:
        indent = len(lines[start]) - len(lines[start].lstrip())
        for i in range(start + 1, len(lines)):
            if lines[i].strip() and not lines[i].startswith(" " * (indent + 1)):
                if not lines[i].strip().startswith(("#", "//", "/*", "*")):
                    return i
        return len(lines)

    def _find_class_end(self, lines: List[str], start: int) -> int:
        return self._find_function_end(lines, start)

    def _find_js_function_end(self, lines: List[str], start: int) -> int:
        brace_count = 0
        for i in range(start, len(lines)):
            brace_count += lines[i].count("{") - lines[i].count("}")
            if brace_count == 0 and i > start:
                return i + 1
        return len(lines)

    def _find_struct_end(self, lines: List[str], start: int) -> int:
        return self._find_js_function_end(lines, start)

    async def _find_hotspots(
        self,
        files: List[Dict],
        symbols: List[Symbol],
    ) -> List[ComplexityHotspot]:
        hotspots = []
        complexity_threshold = 10

        for symbol in symbols:
            metrics = await self._calculate_complexity(symbol, files)
            if metrics.cyclomatic_complexity >= complexity_threshold:
                hotspots.append(ComplexityHotspot(
                    file=symbol.file,
                    line=symbol.start_line,
                    symbol=symbol.name,
                    cyclomatic_complexity=metrics.cyclomatic_complexity,
                    cognitive_complexity=metrics.cognitive_complexity,
                ))

        return sorted(hotspots, key=lambda x: -x.cyclomatic_complexity)[:20]

    async def _calculate_complexity(
        self,
        symbol: Symbol,
        files: List[Dict],
    ) -> ComplexityMetrics:
        content = ""
        for f in files:
            if f["path"] == symbol.file:
                lines = f["content"].split("\n")
                if symbol.start_line <= len(lines):
                    end_line = min(symbol.end_line, len(lines))
                    content = "\n".join(lines[symbol.start_line - 1:end_line])
                break

        if not content:
            return ComplexityMetrics()

        cyclomatic = self._count_cyclomatic(content)
        cognitive = self._count_cognitive(content)
        loc = len([l for l in content.split("\n") if l.strip()])

        return ComplexityMetrics(
            cyclomatic_complexity=cyclomatic,
            cognitive_complexity=cognitive,
            lines_of_code=loc,
        )

    def _count_cyclomatic(self, content: str) -> int:
        keywords = [
            "if ", "elif ", "else:", "for ", "while ",
            "case ", "catch ", "except ", "finally:",
            "and ", "or ", "&&", "||", "?",
        ]
        count = 1
        for kw in keywords:
            count += content.count(kw)
        return count

    def _count_cognitive(self, content: str) -> int:
        lines = content.split("\n")
        complexity = 0
        nesting = 0

        for line in lines:
            stripped = line.strip()

            if any(stripped.startswith(kw) for kw in ["if ", "elif ", "for ", "while ", "case "]):
                complexity += 1 + nesting
                nesting += 1
            elif stripped.startswith("else") or stripped.startswith("except"):
                complexity += 1 + max(0, nesting - 1)
            elif stripped in ["break", "continue"]:
                complexity += 1
            elif stripped in ["}", "endif", "end"]:
                nesting = max(0, nesting - 1)

        return complexity
