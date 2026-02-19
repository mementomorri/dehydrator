"""
Data models shared between Go CLI and Python sidecar.
"""

from typing import Optional, List, Dict, Any
from pydantic import BaseModel, Field
from datetime import datetime
from enum import Enum


class Language(str, Enum):
    PYTHON = "python"
    JAVASCRIPT = "javascript"
    TYPESCRIPT = "typescript"
    GO = "go"
    UNKNOWN = "unknown"


class FileInfo(BaseModel):
    path: str
    content: str
    hash: Optional[str] = None


class Symbol(BaseModel):
    name: str
    type: str
    file: str
    start_line: int
    end_line: int
    signature: Optional[str] = None
    references: List[str] = Field(default_factory=list)


class ComplexityMetrics(BaseModel):
    cyclomatic_complexity: int = 0
    cognitive_complexity: int = 0
    lines_of_code: int = 0
    maintainability_index: float = 0.0
    halstead_difficulty: float = 0.0


class CodeBlock(BaseModel):
    id: str
    file: str
    start_line: int
    end_line: int
    content: str
    language: Language
    symbol_type: str
    symbol_name: str
    metrics: ComplexityMetrics
    embedding: Optional[List[float]] = None


class DuplicateGroup(BaseModel):
    id: str
    blocks: List[CodeBlock]
    similarity: float
    suggested_fix: Optional[str] = None


class FileChange(BaseModel):
    path: str
    original: str
    modified: str
    description: str


class RefactorPlan(BaseModel):
    session_id: str
    changes: List[FileChange]
    description: str
    pattern: Optional[str] = None
    created_at: datetime = Field(default_factory=datetime.now)


class RefactorResult(BaseModel):
    session_id: str
    success: bool
    changes: List[FileChange]
    tests_passed: bool
    error: Optional[str] = None
    metrics_before: ComplexityMetrics
    metrics_after: ComplexityMetrics


class PatternApplied(BaseModel):
    pattern: str
    files: List[str]
    description: str


class MetricsDelta(BaseModel):
    cyclomatic_complexity_delta: int = 0
    cognitive_complexity_delta: int = 0
    maintainability_index_delta: float = 0.0


class Report(BaseModel):
    session_id: str
    generated_at: datetime = Field(default_factory=datetime.now)
    loc_before: int
    loc_after: int
    loc_reduced: int
    duplicates_found: int
    patterns_applied: List[PatternApplied] = Field(default_factory=list)
    files_modified: List[str]
    metrics_delta: MetricsDelta


class ComplexityHotspot(BaseModel):
    file: str
    line: int
    symbol: str
    cyclomatic_complexity: int
    cognitive_complexity: int


class AnalyzeResult(BaseModel):
    total_files: int
    total_symbols: int
    hotspots: List[ComplexityHotspot]
    duplicates: List[DuplicateGroup]
    symbols: List[Symbol]


class ModelTier(str, Enum):
    LIGHT = "light"
    MEDIUM = "medium"
    HEAVY = "heavy"


class ModelConfig(BaseModel):
    local_model: str = ""
    remote_model: str = ""
    provider: str = "ollama"
    api_key: Optional[str] = None
    base_url: Optional[str] = None


class AnalyzeRequest(BaseModel):
    path: str
    files: List[FileInfo] = Field(default_factory=list)
    config: Dict[str, Any] = Field(default_factory=dict)


class DeduplicateRequest(BaseModel):
    path: str
    files: List[FileInfo] = Field(default_factory=list)
    similarity_threshold: float = 0.85


class IdiomatizeRequest(BaseModel):
    path: str
    files: List[FileInfo] = Field(default_factory=list)
    language: Language = Language.PYTHON


class PatternRequest(BaseModel):
    pattern: str
    path: str
    files: List[FileInfo] = Field(default_factory=list)


class ApplyPlanRequest(BaseModel):
    session_id: str


class EmbedRequest(BaseModel):
    files: List[FileInfo]


class APIResponse(BaseModel):
    status: str = "success"
    data: Any
    error: Optional[str] = None
