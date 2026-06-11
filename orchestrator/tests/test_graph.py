from mac_orchestrator.graph import build_graph
from tests.test_nodes import CODER_REPLY, FakeDB, FakeModel


def _compiled():
    models = {"planner": FakeModel("plan"), "coder": FakeModel(CODER_REPLY)}
    return build_graph(models, FakeDB(), project_id="p1", session_id="s1")


def test_graph_has_all_nodes():
    graph = _compiled()
    nodes = set(graph.get_graph().nodes)
    assert {"planner", "coder", "refactor", "apply", "test_runner"} <= nodes


def test_graph_interrupts_before_apply(tmp_path):
    graph = _compiled()
    config = {"configurable": {"thread_id": "t1"}}
    graph.invoke({"project_root": str(tmp_path), "task": "demo",
                  "messages": [], "step_order": 0}, config)
    snapshot = graph.get_state(config)
    assert snapshot.next == ("apply",)
    assert snapshot.values["pending_changes"]  # diff content available for HIL
