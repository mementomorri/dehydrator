"""
Unit tests for the Pattern agent.
Tests design pattern detection and application without LLM calls.
"""

import pytest

from ai_sidecar.agents.pattern import PatternAgent
from ai_sidecar.models import Language, PatternRequest


class TestPatternAgent:
    """Test the pattern agent's rule-based detection."""

    @pytest.fixture
    def agent(self):
        return PatternAgent()

    def test_agent_initialization(self, agent):
        assert agent.llm is None
        assert agent.mcp is None
        assert agent._session_plans == {}

    def test_has_complex_conditionals_true(self, agent):
        content = '''def process(type):
    if type == 'a':
        return 1
    elif type == 'b':
        return 2
    elif type == 'c':
        return 3
    elif type == 'd':
        return 4
    else:
        return 0
'''
        assert agent._has_complex_conditionals(content) is True

    def test_has_complex_conditionals_false(self, agent):
        content = '''def process(x):
    if x > 0:
        return x
    return 0
'''
        assert agent._has_complex_conditionals(content) is False

    def test_has_conditional_instantiation_true(self, agent):
        content = '''def create_handler(type):
    if type == 'http':
        return HttpHandler()
    elif type == 'grpc':
        return GrpcHandler()
'''
        assert agent._has_conditional_instantiation(content) is True

    def test_has_conditional_instantiation_false(self, agent):
        content = '''def process(x):
    if x > 0:
        return x
'''
        assert agent._has_conditional_instantiation(content) is False

    def test_has_event_handling_true(self, agent):
        content = '''class EventEmitter:
    def emit(self, event):
        pass
'''
        assert agent._has_event_handling(content) is True

        content2 = '''def notify_users():
    pass
'''
        assert agent._has_event_handling(content2) is True

    def test_has_event_handling_false(self, agent):
        content = '''def process(x):
    return x * 2
'''
        assert agent._has_event_handling(content) is False

    def test_has_global_state_true(self, agent):
        content = '''_instance = None

def get_instance():
    global _instance
    if _instance is None:
        _instance = Object()
    return _instance
'''
        assert agent._has_global_state(content) is True

    def test_has_global_state_false(self, agent):
        content = '''class Config:
    def __init__(self):
        self.data = {}
'''
        assert agent._has_global_state(content) is False

    def test_extract_module_name(self, agent):
        assert agent._extract_module_name("/path/to/module.py") == "module"
        assert agent._extract_module_name("simple.py") == "simple"
        assert agent._extract_module_name("complex_name.py") == "complex_name"

    def test_generate_strategy_template(self, agent):
        template = agent._generate_strategy_template("/path/to/payment.py")
        assert "PaymentStrategy" in template
        assert "ABC" in template
        assert "@abstractmethod" in template
        assert "Context" in template

    def test_generate_factory_template(self, agent):
        template = agent._generate_factory_template("/path/to/handler.py")
        assert "HandlerFactory" in template
        assert "_registry" in template
        assert "@classmethod" in template
        assert "create" in template

    def test_generate_observer_template(self, agent):
        template = agent._generate_observer_template("/path/to/event.py")
        assert "Observer" in template
        assert "Subject" in template
        assert "attach" in template
        assert "notify" in template

    def test_generate_singleton_template(self, agent):
        template = agent._generate_singleton_template("/path/to/config.py")
        assert "Singleton" in template
        assert "_instance" in template
        assert "__new__" in template
        assert "get_instance" in template


class TestPatternApplication:
    """Test pattern application methods."""

    @pytest.fixture
    def agent(self):
        return PatternAgent()

    def test_apply_strategy_pattern(self, agent):
        content = '''def process(type):
    if type == 'a':
        return 1
    elif type == 'b':
        return 2
    elif type == 'c':
        return 3
    elif type == 'd':
        return 4
    elif type == 'e':
        return 5
'''
        changes = agent._apply_strategy_pattern(content, "processor.py")
        assert len(changes) >= 1
        assert "Strategy" in changes[0].description

    def test_apply_strategy_pattern_no_match(self, agent):
        content = "def simple(): return 1"
        changes = agent._apply_strategy_pattern(content, "simple.py")
        assert len(changes) == 0

    def test_apply_factory_pattern(self, agent):
        content = '''def create_handler(type):
    if type == 'http':
        return HttpHandler()
    elif type == 'grpc':
        return GrpcHandler()
'''
        changes = agent._apply_factory_pattern(content, "handler.py")
        assert len(changes) >= 1
        assert "Factory" in changes[0].description

    def test_apply_factory_pattern_no_match(self, agent):
        content = "def simple(): return Simple()"
        changes = agent._apply_factory_pattern(content, "simple.py")
        assert len(changes) == 0

    def test_apply_observer_pattern(self, agent):
        content = '''class EventEmitter:
    def emit(self, event):
        for callback in self.callbacks:
            callback(event)
'''
        changes = agent._apply_observer_pattern(content, "emitter.py")
        assert len(changes) >= 1
        assert "Observer" in changes[0].description

    def test_apply_observer_pattern_no_match(self, agent):
        content = "class Simple: pass"
        changes = agent._apply_observer_pattern(content, "simple.py")
        assert len(changes) == 0

    def test_apply_singleton_pattern(self, agent):
        content = '''_instance = None

def get_instance():
    global _instance
    if _instance is None:
        _instance = Config()
    return _instance
'''
        changes = agent._apply_singleton_pattern(content, "config.py")
        assert len(changes) >= 1
        assert "Singleton" in changes[0].description

    def test_apply_singleton_pattern_no_match(self, agent):
        content = "class Config: pass"
        changes = agent._apply_singleton_pattern(content, "config.py")
        assert len(changes) == 0


class TestPatternRequest:
    """Test PatternRequest model."""

    def test_request_creation(self):
        request = PatternRequest(
            path="/test",
            files=[{"path": "test.py", "content": "x = 1"}],
            pattern="strategy",
        )
        assert request.path == "/test"
        assert len(request.files) == 1
        assert request.pattern == "strategy"

    def test_request_empty_pattern(self):
        request = PatternRequest(
            path="/test",
            files=[],
            pattern="",
        )
        assert request.pattern == ""


class TestPatternAgentIntegration:
    """Integration tests for pattern agent with full flow."""

    @pytest.fixture
    def agent(self):
        return PatternAgent()

    @pytest.mark.asyncio
    async def test_apply_strategy_full_request(self, agent):
        request = PatternRequest(
            path="/test",
            files=[
                {
                    "path": "payment.py",
                    "content": '''def process_payment(type, amount):
    if type == 'credit':
        return process_credit(amount)
    elif type == 'debit':
        return process_debit(amount)
    elif type == 'paypal':
        return process_paypal(amount)
    elif type == 'bank':
        return process_bank(amount)
    else:
        raise ValueError("Unknown type")
''',
                }
            ],
            pattern="strategy",
        )
        plan = await agent.apply_pattern(request)

        assert plan.session_id is not None
        assert len(plan.changes) >= 1
        assert plan.pattern == "strategy"
        assert agent.get_plan(plan.session_id) is plan

    @pytest.mark.asyncio
    async def test_apply_factory_full_request(self, agent):
        request = PatternRequest(
            path="/test",
            files=[
                {
                    "path": "factory.py",
                    "content": '''def create_handler(type):
    if type == 'http':
        return HttpHandler()
    elif type == 'grpc':
        return GrpcHandler()
''',
                }
            ],
            pattern="factory",
        )
        plan = await agent.apply_pattern(request)

        assert len(plan.changes) >= 1
        assert plan.pattern == "factory"

    @pytest.mark.asyncio
    async def test_auto_detect_patterns(self, agent):
        request = PatternRequest(
            path="/test",
            files=[
                {
                    "path": "complex.py",
                    "content": '''def process(type):
    if type == 'a':
        return 1
    elif type == 'b':
        return 2
    elif type == 'c':
        return 3
    elif type == 'd':
        return 4
    elif type == 'e':
        return 5
''',
                }
            ],
            pattern="",
        )
        plan = await agent.apply_pattern(request)

        assert len(plan.changes) >= 1
        assert "auto-detect" in plan.pattern

    @pytest.mark.asyncio
    async def test_apply_custom_pattern(self, agent):
        request = PatternRequest(
            path="/test",
            files=[{"path": "test.py", "content": "x = 1"}],
            pattern="custom_pattern",
        )
        plan = await agent.apply_pattern(request)

        assert plan.pattern == "custom_pattern"
        assert len(plan.changes) == 0

    @pytest.mark.asyncio
    async def test_multiple_files(self, agent):
        request = PatternRequest(
            path="/test",
            files=[
                {
                    "path": "file1.py",
                    "content": '''def process1(type):
    if type == 'a': return 1
    elif type == 'b': return 2
    elif type == 'c': return 3
    elif type == 'd': return 4
    elif type == 'e': return 5
''',
                },
                {
                    "path": "file2.py",
                    "content": '''_instance = None
def get():
    global _instance
    return _instance
''',
                },
            ],
            pattern="strategy",
        )
        plan = await agent.apply_pattern(request)

        assert len(plan.changes) >= 1

    def test_get_plan_not_found(self, agent):
        assert agent.get_plan("nonexistent-session") is None
