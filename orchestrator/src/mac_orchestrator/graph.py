"""Wire the cyclic TDD StateGraph.

planner -> coder -> [interrupt: HIL] -> apply -> test_runner
test_runner routes: back to coder, on to refactor, or END.
"""
from langgraph.checkpoint.memory import MemorySaver
from langgraph.graph import END, StateGraph

from mac_orchestrator.nodes import (
    make_apply, make_coder, make_planner, make_refactor, make_test_runner,
)
from mac_orchestrator.routing import route_after_test
from mac_orchestrator.state import MacAgentState


def build_graph(models: dict, db, project_id: str, session_id: str, embedder=None):
    g = StateGraph(MacAgentState)
    g.add_node("planner", make_planner(models["planner"], db, project_id, session_id, embedder))
    g.add_node("coder", make_coder(models, db, project_id, session_id))
    g.add_node("refactor", make_refactor(models, db, project_id, session_id))
    g.add_node("apply", make_apply())
    g.add_node("test_runner", make_test_runner(db, project_id, session_id))

    g.set_entry_point("planner")
    g.add_edge("planner", "coder")
    g.add_edge("coder", "apply")
    g.add_edge("refactor", "apply")
    g.add_edge("apply", "test_runner")
    g.add_conditional_edges("test_runner", route_after_test,
                            {"coder": "coder", "refactor": "refactor",
                             "done": END, "halt": END})

    # HIL: pause before any disk write so the operator reviews the diff.
    return g.compile(checkpointer=MemorySaver(), interrupt_before=["apply"])
