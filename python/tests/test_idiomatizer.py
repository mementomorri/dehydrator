"""
Unit tests for the Idiomatizer agent.
Tests rule-based idiom detection without LLM calls.
"""

import pytest

from ai_sidecar.agents.idiomatizer import IdiomatizerAgent
from ai_sidecar.models import Language, IdiomatizeRequest


class TestIdiomatizerAgent:
    """Test the idiomatizer agent's rule-based detection."""

    @pytest.fixture
    def agent(self):
        return IdiomatizerAgent()

    def test_agent_initialization(self, agent):
        assert agent.llm is None
        assert agent.mcp is None
        assert agent._session_plans == {}

    def test_is_for_append_pattern_true(self, agent):
        lines = [
            "result = []",
            "for x in items:",
            "    result.append(x * 2)",
            "print(result)",
        ]
        assert agent._is_for_append_pattern(lines, 1) is True

    def test_is_for_append_pattern_false_no_append(self, agent):
        lines = [
            "for x in items:",
            "    print(x)",
        ]
        assert agent._is_for_append_pattern(lines, 0) is False

    def test_is_for_append_pattern_false_not_for(self, agent):
        lines = [
            "result = []",
            "result.append(1)",
        ]
        assert agent._is_for_append_pattern(lines, 1) is False

    def test_convert_to_list_comp_simple(self, agent):
        lines = [
            "result = []",
            "for x in items:",
            "    result.append(x * 2)",
        ]
        result = agent._convert_to_list_comp(lines, 1)
        assert result is not None
        original, modified, description = result
        assert "for x in items:" in original
        assert "result.append(x * 2)" in original
        assert "result = [x * 2 for x in items]" in modified
        assert "list comprehension" in description.lower()

    def test_convert_to_list_comp_with_condition(self, agent):
        lines = [
            "evens = []",
            "for n in numbers:",
            "    evens.append(n)",
        ]
        result = agent._convert_to_list_comp(lines, 1)
        assert result is not None
        original, modified, _ = result
        assert "for n in numbers:" in original

    def test_convert_to_list_comp_nested_condition(self, agent):
        lines = [
            "evens = []",
            "for n in numbers:",
            "    if n % 2 == 0:",
            "        evens.append(n)",
        ]
        result = agent._is_for_append_pattern(lines, 1)
        assert result is False

    def test_is_dict_init_pattern_true(self, agent):
        assert agent._is_dict_init_pattern("result = {}") is True
        assert agent._is_dict_init_pattern("data = {}") is True

    def test_is_dict_init_pattern_false(self, agent):
        assert agent._is_dict_init_pattern("if x:") is False
        assert agent._is_dict_init_pattern("result = []") is False

    def test_is_string_concat_pattern_true(self, agent):
        lines = ['message = "Hello, " + name + "!"']
        assert agent._is_string_concat_pattern(lines, 0) is True

    def test_is_string_concat_pattern_false(self, agent):
        lines = ["x = 1 + 2"]
        assert agent._is_string_concat_pattern(lines, 0) is False

    def test_idiomatize_python_for_append(self, agent):
        content = '''result = []
for x in items:
    result.append(x * 2)
'''
        changes = agent._idiomatize_python(content, "test.py")
        assert len(changes) >= 1
        assert "list comprehension" in changes[0].description.lower()

    def test_idiomatize_python_multiple_patterns(self, agent):
        content = '''result = []
for x in items:
    result.append(x * 2)

message = "Hello, " + name + "!"
'''
        changes = agent._idiomatize_python(content, "test.py")
        assert len(changes) >= 1

    def test_idiomatize_python_no_changes(self, agent):
        content = '''def hello():
    return "Hello, World!"
'''
        changes = agent._idiomatize_python(content, "test.py")
        assert len(changes) == 0

    def test_idiomatize_js_var_to_const(self, agent):
        content = "var x = 1;"
        changes = agent._idiomatize_js(content, "test.js", Language.JAVASCRIPT)
        assert len(changes) >= 1
        assert "const" in changes[0].modified or "let" in changes[0].description

    def test_idiomatize_js_for_loop(self, agent):
        content = "for (var i = 0; i < arr.length; i++) { console.log(arr[i]); }"
        changes = agent._idiomatize_js(content, "test.js", Language.JAVASCRIPT)
        assert len(changes) >= 1

    def test_idiomatize_js_no_changes(self, agent):
        content = "const x = 1;"
        changes = agent._idiomatize_js(content, "test.js", Language.JAVASCRIPT)
        assert len(changes) == 0

    def test_idiomatize_go_basic(self, agent):
        content = '''package main

func main() {
    if err != nil {
        return err
    }
}
'''
        changes = agent._idiomatize_go(content, "test.go")
        assert isinstance(changes, list)

    def test_try_python_idiom_for_append(self, agent):
        lines = [
            "result = []",
            "for x in items:",
            "    result.append(x * 2)",
        ]
        result = agent._try_python_idiom(lines[1], lines, 1)
        assert result is not None

    def test_try_python_idiom_no_match(self, agent):
        lines = ["def hello():", "    pass"]
        result = agent._try_python_idiom(lines[1], lines, 1)
        assert result is None


class TestIdiomatizerRequest:
    """Test IdiomatizeRequest model."""

    def test_request_creation(self):
        request = IdiomatizeRequest(
            path="/test",
            files=[{"path": "test.py", "content": "x = 1"}],
            language=Language.PYTHON,
        )
        assert request.path == "/test"
        assert len(request.files) == 1
        assert request.language == Language.PYTHON


class TestIdiomatizerIntegration:
    """Integration tests for idiomatizer with full flow."""

    @pytest.fixture
    def agent(self):
        return IdiomatizerAgent()

    @pytest.mark.asyncio
    async def test_idiomatize_full_request(self, agent):
        request = IdiomatizeRequest(
            path="/test",
            files=[
                {
                    "path": "test.py",
                    "content": "result = []\nfor x in items:\n    result.append(x * 2)\n",
                }
            ],
            language=Language.PYTHON,
        )
        plan = await agent.idiomatize(request)

        assert plan.session_id is not None
        assert len(plan.changes) >= 1
        assert "idiomatic" in plan.description.lower()
        assert agent.get_plan(plan.session_id) is plan

    @pytest.mark.asyncio
    async def test_idiomatize_multiple_files(self, agent):
        request = IdiomatizeRequest(
            path="/test",
            files=[
                {
                    "path": "file1.py",
                    "content": "result = []\nfor x in items:\n    result.append(x)\n",
                },
                {
                    "path": "file2.js",
                    "content": "var x = 1;",
                },
            ],
            language=Language.PYTHON,
        )
        plan = await agent.idiomatize(request)

        assert len(plan.changes) >= 1

    @pytest.mark.asyncio
    async def test_idiomatize_empty_files(self, agent):
        request = IdiomatizeRequest(
            path="/test",
            files=[],
            language=Language.PYTHON,
        )
        plan = await agent.idiomatize(request)

        assert len(plan.changes) == 0

    def test_get_plan_not_found(self, agent):
        assert agent.get_plan("nonexistent-session") is None
