import pytest

from apps.api.errors import DatasetServiceError
from apps.api.services import (
    create_profile_dataset,
    create_profile_project,
    patch_profile_dataset_row_by_index,
)
from apps.core.activation import record_profile_activation_milestone
from apps.core.analytics import ACTIVATION_FUNNEL
from apps.core.choices import ActivationMilestoneType
from apps.core.models import ProfileActivationMilestone
from apps.core.services import create_agent_api_key

pytestmark = pytest.mark.django_db


def test_activation_funnel_has_one_ordered_canonical_event_per_stage():
    assert [stage.order for stage in ACTIVATION_FUNNEL] == list(
        range(1, len(ACTIVATION_FUNNEL) + 1)
    )
    assert len({stage.name for stage in ACTIVATION_FUNNEL}) == len(ACTIVATION_FUNNEL)
    assert len({stage.event_name for stage in ACTIVATION_FUNNEL}) == len(ACTIVATION_FUNNEL)


def test_reported_activation_milestone_is_idempotent_and_privacy_safe(profile, monkeypatch):
    tracked = []
    monkeypatch.setattr(
        "apps.core.activation.track_activation_event",
        lambda profile, event_name, properties, **kwargs: tracked.append(
            (profile.id, event_name, properties, kwargs)
        ),
    )

    first = record_profile_activation_milestone(
        profile,
        ActivationMilestoneType.RECOMMENDATION_EMITTED,
        interface="mcp",
    )
    retry = record_profile_activation_milestone(
        profile,
        ActivationMilestoneType.RECOMMENDATION_EMITTED,
        interface="mcp",
    )

    assert first["recorded"] is True
    assert retry["recorded"] is False
    assert (
        ProfileActivationMilestone.objects.filter(
            profile=profile,
            milestone=ActivationMilestoneType.RECOMMENDATION_EMITTED,
        ).count()
        == 1
    )
    assert tracked == [
        (
            profile.id,
            "rowset_personalized_recommendation_emitted",
            {
                "activation_stage": "personalized_recommendation_emitted",
                "activation_stage_order": 6,
                "interface": "mcp",
            },
            {"source_function": "record_profile_activation_milestone"},
        )
    ]
    assert "recommendation" not in tracked[0][2]
    assert set(tracked[0][2]) == {
        "activation_stage",
        "activation_stage_order",
        "interface",
    }


def test_agent_first_value_events_are_ordered_and_require_verified_index_update(
    profile,
    monkeypatch,
):
    agent_api_key = create_agent_api_key(profile, "Activation test").agent_api_key
    tracked = []
    monkeypatch.setattr(
        "apps.core.activation.track_activation_event",
        lambda profile, event_name, properties, **_kwargs: tracked.append((event_name, properties)),
    )
    monkeypatch.setattr("apps.api.services.track_activation_event", lambda *_args, **_kwargs: None)

    record_profile_activation_milestone(
        profile,
        ActivationMilestoneType.RECOMMENDATION_EMITTED,
        interface="rest",
    )
    record_profile_activation_milestone(
        profile,
        ActivationMilestoneType.RECOMMENDATION_ACCEPTED,
        interface="rest",
    )
    project = create_profile_project(
        profile,
        name="Private customer workflow",
        description="Contains confidential work.",
        agent_api_key=agent_api_key,
    )
    dataset = create_profile_dataset(
        profile,
        name="Private task board",
        headers=["task_id", "private_notes"],
        rows=[{"task_id": "TASK-1", "private_notes": "Do not track this"}],
        index_column="task_id",
        project_key=project["project"]["key"],
        agent_api_key=agent_api_key,
    )
    create_profile_project(
        profile,
        name="Second customer workflow",
        agent_api_key=agent_api_key,
    )
    create_profile_dataset(
        profile,
        name="Second private task board",
        headers=["task_id", "status"],
        rows=[],
        index_column="task_id",
        project_key=project["project"]["key"],
        agent_api_key=agent_api_key,
    )

    assert not ProfileActivationMilestone.objects.filter(
        profile=profile,
        milestone=ActivationMilestoneType.FIRST_VERIFIED_INDEXED_ROW_UPDATE,
    ).exists()
    assert (
        ProfileActivationMilestone.objects.filter(
            profile=profile,
            milestone__in=[
                ActivationMilestoneType.FIRST_PROJECT_CREATED,
                ActivationMilestoneType.FIRST_DATASET_CREATED,
            ],
        ).count()
        == 2
    )
    assert [event_name for event_name, _properties in tracked] == [
        "rowset_personalized_recommendation_emitted",
        "rowset_personalized_recommendation_accepted",
        "rowset_first_project_created",
        "rowset_first_dataset_created",
    ]

    with pytest.raises(DatasetServiceError, match="Row not found"):
        patch_profile_dataset_row_by_index(
            profile,
            dataset["dataset"]["key"],
            "TASK-MISSING",
            {"private_notes": "Must stay untracked"},
            agent_api_key=agent_api_key,
        )

    assert not ProfileActivationMilestone.objects.filter(
        profile=profile,
        milestone=ActivationMilestoneType.FIRST_VERIFIED_INDEXED_ROW_UPDATE,
    ).exists()
    assert len(tracked) == 4

    updated = patch_profile_dataset_row_by_index(
        profile,
        dataset["dataset"]["key"],
        "TASK-1",
        {"private_notes": "Still private"},
        agent_api_key=agent_api_key,
    )
    retried = patch_profile_dataset_row_by_index(
        profile,
        dataset["dataset"]["key"],
        "TASK-1",
        {"private_notes": "Still private"},
        agent_api_key=agent_api_key,
    )

    assert updated["row"]["data"]["private_notes"] == "Still private"
    assert retried["row"]["data"]["private_notes"] == "Still private"
    assert [event_name for event_name, _properties in tracked] == [
        "rowset_personalized_recommendation_emitted",
        "rowset_personalized_recommendation_accepted",
        "rowset_first_project_created",
        "rowset_first_dataset_created",
        "rowset_first_verified_indexed_row_update",
    ]
    assert [properties["activation_stage_order"] for _event_name, properties in tracked] == [
        6,
        7,
        8,
        9,
        10,
    ]
    assert "Private customer workflow" not in str(tracked)
    assert "Private task board" not in str(tracked)
    assert "Do not track this" not in str(tracked)
    assert "Still private" not in str(tracked)


def test_internal_and_by_id_writes_do_not_claim_verified_agent_first_value(
    profile,
    monkeypatch,
):
    tracked = []
    monkeypatch.setattr(
        "apps.core.activation.track_activation_event",
        lambda *_args, **kwargs: tracked.append(kwargs),
    )
    monkeypatch.setattr("apps.api.services.track_activation_event", lambda *_args, **_kwargs: None)

    dataset = create_profile_dataset(
        profile,
        name="Internal tasks",
        headers=["task_id", "status"],
        rows=[{"task_id": "TASK-1", "status": "Todo"}],
        index_column="task_id",
    )
    patch_profile_dataset_row_by_index(
        profile,
        dataset["dataset"]["key"],
        "TASK-1",
        {"status": "Done"},
    )

    assert tracked == []
    assert not ProfileActivationMilestone.objects.filter(
        profile=profile,
        milestone=ActivationMilestoneType.FIRST_VERIFIED_INDEXED_ROW_UPDATE,
    ).exists()
