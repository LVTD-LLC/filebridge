REGISTRATION_COMPLETED_SESSION_KEY = "openai_ads_registration_completed"


def mark_registration_completed(request) -> None:
    session = getattr(request, "session", None)
    if session is not None:
        session[REGISTRATION_COMPLETED_SESSION_KEY] = True
