"""
Pattern agent for applying design patterns.
"""

import uuid
from typing import List, Dict, Optional

from ai_sidecar.models import (
    PatternRequest,
    RefactorPlan,
    FileChange,
    Language,
    ModelTier,
)


class PatternAgent:
    def __init__(self, llm_router=None, mcp_client=None):
        self.llm = llm_router
        self.mcp = mcp_client
        self._session_plans: Dict[str, RefactorPlan] = {}

    async def apply_pattern(self, request: PatternRequest) -> RefactorPlan:
        pattern = request.pattern.lower()
        path = request.path
        files = request.files

        changes = []

        if pattern in ("strategy", "factory", "observer", "singleton"):
            changes = await self._apply_design_pattern(files, pattern)
        elif pattern == "":
            changes = await self._detect_and_suggest_patterns(files)
        else:
            changes = await self._apply_custom_pattern(files, pattern)

        session_id = str(uuid.uuid4())
        plan = RefactorPlan(
            session_id=session_id,
            changes=changes,
            description=f"Applied {pattern or 'detected'} design patterns to {len(changes)} locations.",
            pattern=pattern if pattern else "auto-detect",
        )

        self._session_plans[session_id] = plan
        return plan

    async def _apply_design_pattern(
        self,
        files,
        pattern: str,
    ) -> List[FileChange]:
        changes = []

        for file in files:
            if hasattr(file, 'content'):
                content = file.content
                path = file.path
            else:
                content = file["content"]
                path = file["path"]

            if pattern == "strategy":
                file_changes = self._apply_strategy_pattern(content, path)
            elif pattern == "factory":
                file_changes = self._apply_factory_pattern(content, path)
            elif pattern == "observer":
                file_changes = self._apply_observer_pattern(content, path)
            elif pattern == "singleton":
                file_changes = self._apply_singleton_pattern(content, path)
            else:
                file_changes = []

            changes.extend(file_changes)

        return changes

    def _apply_strategy_pattern(self, content: str, path: str) -> List[FileChange]:
        changes = []

        if self._has_complex_conditionals(content):
            strategy_code = self._generate_strategy_template(path)
            changes.append(FileChange(
                path=f"strategies/{self._extract_module_name(path)}_strategy.py",
                original="",
                modified=strategy_code,
                description="Extract conditional logic into Strategy pattern",
            ))

        return changes

    def _apply_factory_pattern(self, content: str, path: str) -> List[FileChange]:
        changes = []

        if self._has_conditional_instantiation(content):
            factory_code = self._generate_factory_template(path)
            changes.append(FileChange(
                path=f"factories/{self._extract_module_name(path)}_factory.py",
                original="",
                modified=factory_code,
                description="Extract conditional instantiation into Factory pattern",
            ))

        return changes

    def _apply_observer_pattern(self, content: str, path: str) -> List[FileChange]:
        changes = []

        if self._has_event_handling(content):
            observer_code = self._generate_observer_template(path)
            changes.append(FileChange(
                path=f"observers/{self._extract_module_name(path)}_observer.py",
                original="",
                modified=observer_code,
                description="Extract event handling into Observer pattern",
            ))

        return changes

    def _apply_singleton_pattern(self, content: str, path: str) -> List[FileChange]:
        changes = []

        if self._has_global_state(content):
            singleton_code = self._generate_singleton_template(path)
            changes.append(FileChange(
                path=path,
                original=content,
                modified=singleton_code,
                description="Wrap global state in Singleton pattern",
            ))

        return changes

    async def _detect_and_suggest_patterns(self, files) -> List[FileChange]:
        changes = []

        for file in files:
            if hasattr(file, 'content'):
                content = file.content
                path = file.path
            else:
                content = file["content"]
                path = file["path"]

            if self._has_complex_conditionals(content):
                changes.append(FileChange(
                    path=path,
                    original="",
                    modified=self._generate_strategy_template(path),
                    description="Suggest Strategy pattern for complex conditionals",
                ))

            if self._has_conditional_instantiation(content):
                changes.append(FileChange(
                    path=path,
                    original="",
                    modified=self._generate_factory_template(path),
                    description="Suggest Factory pattern for conditional instantiation",
                ))

        return changes

    async def _apply_custom_pattern(
        self,
        files: List[Dict],
        pattern: str,
    ) -> List[FileChange]:
        return []

    def _has_complex_conditionals(self, content: str) -> bool:
        if_count = content.count("if ")
        elif_count = content.count("elif ")
        return if_count + elif_count >= 5

    def _has_conditional_instantiation(self, content: str) -> bool:
        patterns = ["new ", "= new ", "return new "]
        for pattern in patterns:
            if pattern in content and "if " in content:
                return True
        import re
        if "if " in content:
            if re.search(r'return\s+\w+Handler\(\)', content):
                return True
            if re.search(r'return\s+\w+Factory\(\)', content):
                return True
            if re.search(r'return\s+\w+Client\(\)', content):
                return True
            if re.search(r'return\s+\w+\(\)', content) and "elif" in content:
                return True
        return False

    def _has_event_handling(self, content: str) -> bool:
        event_keywords = ["emit", "trigger", "dispatch", "notify", "subscribe", "on_"]
        return any(kw in content.lower() for kw in event_keywords)

    def _has_global_state(self, content: str) -> bool:
        return "global " in content

    def _extract_module_name(self, path: str) -> str:
        import os
        basename = os.path.basename(path)
        name, _ = os.path.splitext(basename)
        return name

    def _generate_strategy_template(self, path: str) -> str:
        module = self._extract_module_name(path)
        return f'''"""
Strategy pattern implementation for {module}.
"""

from abc import ABC, abstractmethod
from typing import Any


class {module.title()}Strategy(ABC):
    """Abstract strategy interface."""
    
    @abstractmethod
    def execute(self, *args, **kwargs) -> Any:
        """Execute the strategy."""
        pass


class Context:
    """Context that uses a strategy."""
    
    def __init__(self, strategy: {module.title()}Strategy):
        self._strategy = strategy
    
    def set_strategy(self, strategy: {module.title()}Strategy) -> None:
        self._strategy = strategy
    
    def execute_strategy(self, *args, **kwargs) -> Any:
        return self._strategy.execute(*args, **kwargs)
'''

    def _generate_factory_template(self, path: str) -> str:
        module = self._extract_module_name(path)
        return f'''"""
Factory pattern implementation for {module}.
"""

from typing import Any, Dict, Type


class {module.title()}Factory:
    """Factory for creating {module} instances."""
    
    _registry: Dict[str, Type] = {{}}
    
    @classmethod
    def register(cls, name: str, type_class: Type) -> None:
        """Register a type with the factory."""
        cls._registry[name] = type_class
    
    @classmethod
    def create(cls, name: str, *args, **kwargs) -> Any:
        """Create an instance by name."""
        if name not in cls._registry:
            raise ValueError(f"Unknown type: {{name}}")
        return cls._registry[name](*args, **kwargs)
'''

    def _generate_observer_template(self, path: str) -> str:
        module = self._extract_module_name(path)
        return f'''"""
Observer pattern implementation for {module}.
"""

from typing import List, Callable


class Observer:
    """Observer interface."""
    
    def update(self, subject: "Subject", *args, **kwargs) -> None:
        """Called when subject state changes."""
        pass


class Subject:
    """Subject that notifies observers."""
    
    def __init__(self):
        self._observers: List[Observer] = []
    
    def attach(self, observer: Observer) -> None:
        """Attach an observer."""
        self._observers.append(observer)
    
    def detach(self, observer: Observer) -> None:
        """Detach an observer."""
        self._observers.remove(observer)
    
    def notify(self, *args, **kwargs) -> None:
        """Notify all observers."""
        for observer in self._observers:
            observer.update(self, *args, **kwargs)
'''

    def _generate_singleton_template(self, path: str) -> str:
        return '''"""
Singleton pattern implementation.
"""


class Singleton:
    """Singleton base class."""
    
    _instance = None
    
    def __new__(cls):
        if cls._instance is None:
            cls._instance = super().__new__(cls)
        return cls._instance
    
    @classmethod
    def get_instance(cls):
        """Get the singleton instance."""
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
'''

    def get_plan(self, session_id: str) -> Optional[RefactorPlan]:
        return self._session_plans.get(session_id)
