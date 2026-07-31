import pytest

from apps.core import capabilities
from apps.core.capabilities import (
    CapabilitySelectionError,
    RowsetCapabilityTopic,
    RowsetUseCase,
    rowset_capabilities_payload,
)


def test_use_case_feature_references_match_registered_capability_ids():
    payload = rowset_capabilities_payload(full=True, include_use_cases=True)
    capability_ids = {capability["id"] for capability in payload["capabilities"]}

    for use_case in payload["use_cases"]:
        assert set(use_case["rowset_features"]) <= capability_ids


def test_capabilities_payload_includes_core_rowset_surfaces():
    payload = rowset_capabilities_payload(full=True)

    assert {capability["id"] for capability in payload["capabilities"]} >= {"rows"}


def test_capabilities_payload_describes_formula_columns():
    payload = rowset_capabilities_payload(topics=["schema"])
    dataset_context = next(
        capability
        for capability in payload["capabilities"]
        if capability["id"] == "dataset_context"
    )
    notes = " ".join(dataset_context["notes"])

    assert '"calculation": "formula"' in notes
    assert "DATEADD" in notes
    assert "NOW" in notes


def test_capabilities_payload_rejects_unknown_use_case_feature_references(monkeypatch):
    monkeypatch.setattr(
        capabilities,
        "ROWSET_USE_CASES",
        capabilities.ROWSET_USE_CASES
        + (
            RowsetUseCase(
                id="invalid_reference",
                title="Invalid reference",
                summary="Invalid registry fixture.",
                starter_shape=("Fixture only.",),
                rowset_features=("missing_capability",),
            ),
        ),
    )

    with pytest.raises(
        ValueError,
        match="invalid_reference: missing_capability",
    ):
        rowset_capabilities_payload(full=True, include_use_cases=True)


def test_capabilities_payload_rejects_duplicate_capability_ids(monkeypatch):
    monkeypatch.setattr(
        capabilities,
        "ROWSET_CAPABILITIES",
        capabilities.ROWSET_CAPABILITIES + (capabilities.ROWSET_CAPABILITIES[0],),
    )

    with pytest.raises(ValueError, match="duplicate IDs"):
        rowset_capabilities_payload(full=True)


def test_capabilities_payload_defaults_to_compact_topic_index():
    payload = rowset_capabilities_payload()

    assert payload["mode"] == "summary"
    assert "capabilities" not in payload
    assert "interfaces" not in payload
    assert "recommended_startup" not in payload
    assert "use_cases" not in payload
    assert {topic["id"] for topic in payload["available_topics"]} >= {
        "rows",
        "relationships",
        "schema",
        "assets",
        "previews",
        "setup",
    }
    assert len(str(payload)) < 3_000


def test_capabilities_payload_returns_only_requested_topics():
    payload = rowset_capabilities_payload(topics=["rows", "schema"])

    assert payload["mode"] == "topics"
    assert payload["requested_topics"] == ["rows", "schema"]
    assert {capability["id"] for capability in payload["capabilities"]} == {
        "rows",
        "dataset_context",
        "schema_mutations",
    }
    assert "interfaces" not in payload
    assert "recommended_startup" not in payload
    assert "use_cases" not in payload


def test_topic_payload_includes_only_fully_supported_use_cases():
    payload = rowset_capabilities_payload(
        topics=["schema", "rows", "projects"],
        include_use_cases=True,
    )

    assert [use_case["id"] for use_case in payload["use_cases"]] == ["task_board"]


def test_capabilities_payload_setup_topic_includes_setup_details():
    payload = rowset_capabilities_payload(topics=["setup"])

    interfaces = {interface["id"]: interface for interface in payload["interfaces"]}
    assert set(interfaces) == {
        "mcp",
        "cli",
        "rest",
    }
    assert (
        "Choose first when the runtime natively supports remote MCP"
        in interfaces["mcp"]["selection_rule"]
    )
    assert "private bearer-secret configuration" in interfaces["mcp"]["selection_rule"]
    assert (
        "native remote MCP or private bearer-secret configuration is unavailable"
        in interfaces["cli"]["selection_rule"]
    )
    assert "code-only or HTTP-only runtimes" in interfaces["rest"]["selection_rule"]
    assert (
        "neither a usable remote MCP configuration nor a trusted terminal workflow is available"
        in interfaces["rest"]["selection_rule"]
    )
    startup = " ".join(payload["recommended_startup"])
    assert "autonomously select the best supported interface" in startup
    assert "Do not ask the user to compare or choose" in startup
    assert "unavoidable operating-system, authentication, or secret-manager" in startup


def test_setup_recovery_contract_handles_interrupted_configuration():
    recovery = rowset_capabilities_payload(topics=["setup"])["setup_recovery"]
    rules = " ".join(recovery["rules"])

    assert recovery["state_machine"] == ["inspect", "choose", "configure", "verify"]
    assert recovery["failure_report_fields"] == [
        "completed_steps",
        "failed_or_cancelled_step",
        "credential_storage_state",
        "verification_status",
        "retry_action",
    ]
    assert recovery["credential_storage_states"] == ["confirmed", "unknown", "absent"]
    assert recovery["retry_action_limit"] == 1
    assert "Cancelled authentication or permission is incomplete" in rules
    assert "inspect existing configuration and secret storage" in rules
    assert "Do not create duplicate configuration" in rules
    assert "rotate or replace credentials" in rules


def test_setup_recovery_contract_handles_failed_verification():
    recovery = rowset_capabilities_payload(topics=["setup"])["setup_recovery"]
    rules = " ".join(recovery["rules"])

    assert recovery["verification_states"] == ["not_run", "failed", "succeeded"]
    assert "Verification must be reported as not_run, failed, or succeeded" in rules
    assert "Only succeeded verification makes setup complete" in rules
    assert "exactly one safe retry action" in rules
    assert "Never expose the credential" in rules


def test_first_project_recommendation_uses_only_authorized_context():
    recommendation = rowset_capabilities_payload(topics=["setup"])["first_project_recommendation"]
    rules = " ".join(recommendation["rules"])

    assert recommendation["project_count"] == 1
    assert recommendation["dataset_count"] == {"minimum": 1, "maximum": 3}
    assert recommendation["output_fields"] == [
        "project_name",
        "dataset_names",
        "evidence_summary",
        "rationale",
    ]
    assert recommendation["authorized_evidence"] == [
        "current conversation",
        "current repository and steering documents",
        "active task description",
        "sources already authorized for the current task",
    ]
    assert "Do not enumerate unrelated private resources" in rules
    assert "Do not create the recommended project or datasets until the user confirms" in rules


def test_first_project_recommendation_covers_code_content_and_weak_context():
    recommendation = rowset_capabilities_payload(topics=["setup"])["first_project_recommendation"]
    examples = recommendation["examples"]

    assert [example["context"] for example in examples] == [
        "code_project",
        "content_workflow",
        "insufficient_context",
    ]
    assert examples[0]["project_name"] == "ReviewGate"
    assert examples[0]["dataset_names"] == [
        "Improvement task board",
        "Agent feedback",
    ]
    assert examples[1]["project_name"] == "Content operations"
    assert examples[1]["dataset_names"] == [
        "Content queue",
        "Research library",
        "Performance tracker",
    ]
    output_fields = set(recommendation["output_fields"])
    for example in examples:
        if "project_name" in example:
            assert output_fields <= set(example)
    assert examples[2]["question"] == (
        "What are you working on that you want Rowset to help organize?"
    )
    assert recommendation["confirmation_question"] == (
        "Would you like me to create that project and those datasets?"
    )
    assert (
        recommendation["automation_offer_timing"]
        == "after the user answers the project confirmation question"
    )


def test_capabilities_payload_makes_use_cases_opt_in():
    without_use_cases = rowset_capabilities_payload()
    with_use_cases = rowset_capabilities_payload(include_use_cases=True)

    assert "use_cases" not in without_use_cases
    assert {use_case["id"] for use_case in with_use_cases["use_cases"]} >= {
        "task_board",
        "bug_tracker",
    }


def test_capabilities_payload_rejects_unknown_topics():
    with pytest.raises(CapabilitySelectionError, match="Unknown capability topic: unknown"):
        rowset_capabilities_payload(topics=["unknown"])


def test_capabilities_payload_rejects_topics_with_full_mode():
    with pytest.raises(CapabilitySelectionError, match="Choose topics or full mode"):
        rowset_capabilities_payload(topics=["rows"], full=True)


def test_capabilities_payload_rejects_topic_references_missing_from_registry(monkeypatch):
    monkeypatch.setattr(
        capabilities,
        "ROWSET_CAPABILITY_TOPICS",
        capabilities.ROWSET_CAPABILITY_TOPICS
        + (
            RowsetCapabilityTopic(
                id="invalid",
                title="Invalid registry fixture",
                capability_ids=("missing_capability",),
            ),
        ),
    )

    with pytest.raises(ValueError, match=r"unknown=\['missing_capability'\]"):
        rowset_capabilities_payload()


def test_capabilities_payload_rejects_capabilities_missing_from_topics(monkeypatch):
    monkeypatch.setattr(
        capabilities,
        "ROWSET_CAPABILITY_TOPICS",
        tuple(topic for topic in capabilities.ROWSET_CAPABILITY_TOPICS if topic.id != "rows"),
    )

    with pytest.raises(ValueError, match=r"missing=\['rows'\]"):
        rowset_capabilities_payload()
