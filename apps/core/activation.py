from typing import Any

from apps.core.analytics import (
    ACTIVATION_FUNNEL_BY_MILESTONE,
    activation_funnel_event_properties,
    track_activation_event,
)
from apps.core.choices import ActivationMilestoneType
from apps.core.models import Profile, ProfileActivationMilestone

ACTIVATION_MILESTONE_INTERFACES = {"mcp", "rest", "server"}


def record_profile_activation_milestone(
    profile: Profile,
    milestone: ActivationMilestoneType | str,
    *,
    interface: str,
) -> dict[str, Any]:
    """Persist and emit a bounded first-occurrence activation milestone."""
    normalized_milestone = ActivationMilestoneType(milestone)
    if interface not in ACTIVATION_MILESTONE_INTERFACES:
        raise ValueError(f"Unsupported activation milestone interface: {interface}.")

    stage = ACTIVATION_FUNNEL_BY_MILESTONE[normalized_milestone]
    _milestone, created = ProfileActivationMilestone.objects.get_or_create(
        profile=profile,
        milestone=normalized_milestone,
    )
    if created:
        track_activation_event(
            profile,
            stage.event_name,
            {
                **activation_funnel_event_properties(stage.event_name),
                "interface": interface,
            },
            source_function="record_profile_activation_milestone",
        )

    return {
        "status": "success",
        "message": "Activation milestone recorded." if created else "Activation milestone exists.",
        "milestone": normalized_milestone.value,
        "recorded": created,
    }
