import pytest
from allauth.account.adapter import DefaultAccountAdapter
from django.contrib.auth import get_user_model
from django.contrib.sessions.middleware import SessionMiddleware
from django.http import HttpResponseRedirect
from django.test import RequestFactory
from django.urls import reverse

from apps.core.openai_ads import REGISTRATION_COMPLETED_SESSION_KEY
from rowset.adapters import CustomAccountAdapter

PIXEL_ID = "8dYXMBukBoC779PidZY6ZX"
pytestmark = pytest.mark.django_db


def test_openai_ads_pixel_is_installed_once_on_public_pages(client, settings):
    settings.OPENAI_ADS_PIXEL_ID = PIXEL_ID
    settings.ENVIRONMENT = "prod"

    response = client.get(reverse("landing"))

    content = response.content.decode()
    assert content.count("https://bzrcdn.openai.com/sdk/oaiq.min.js") == 1
    assert content.count(f'pixelId: "{PIXEL_ID}"') == 1
    assert "debug: true" not in content


def test_openai_ads_pixel_is_disabled_without_configuration(client, settings):
    settings.OPENAI_ADS_PIXEL_ID = ""

    response = client.get(reverse("landing"))

    assert "https://bzrcdn.openai.com/sdk/oaiq.min.js" not in response.content.decode()


def test_successful_signup_measures_registration_once(client, monkeypatch, settings):
    settings.OPENAI_ADS_PIXEL_ID = PIXEL_ID
    settings.ENVIRONMENT = "prod"
    client.cookies["rowset_analytics_consent"] = "granted"

    monkeypatch.setattr(
        "rowset.adapters.CustomAccountAdapter.send_confirmation_mail",
        lambda *_args, **_kwargs: None,
    )

    response = client.post(
        reverse("account_signup"),
        data={
            "email": "openai-pixel-user@example.com",
            "password1": "strong-test-pass-123",
        },
        follow=True,
    )

    assert response.status_code == 200
    assert get_user_model().objects.filter(email="openai-pixel-user@example.com").exists()
    content = response.content.decode()
    assert "registrationCompleted: true" in content
    assert 'oaiq("measure", "registration_completed", {' not in content

    next_response = client.get(reverse("home"))
    assert "registrationCompleted: true" not in next_response.content.decode()


def test_failed_signup_does_not_mark_registration_completed(client, settings):
    settings.OPENAI_ADS_PIXEL_ID = PIXEL_ID

    response = client.post(
        reverse("account_signup"),
        data={"email": "invalid", "password1": "short"},
    )

    assert response.status_code == 200
    assert "registrationCompleted: true" not in response.content.decode()


def test_social_signup_marks_post_login_session(monkeypatch):
    request = RequestFactory().get("/")
    SessionMiddleware(lambda _request: None).process_request(request)
    response = HttpResponseRedirect("/")
    monkeypatch.setattr(DefaultAccountAdapter, "post_login", lambda *_args, **_kwargs: response)

    result = CustomAccountAdapter().post_login(
        request,
        object(),
        email_verification="none",
        signal_kwargs={"sociallogin": object()},
        email="social@example.com",
        signup=True,
        redirect_url="/",
    )

    assert result is response
    assert request.session[REGISTRATION_COMPLETED_SESSION_KEY] is True


def test_existing_user_login_does_not_mark_registration(monkeypatch):
    request = RequestFactory().get("/")
    SessionMiddleware(lambda _request: None).process_request(request)
    response = HttpResponseRedirect("/")
    monkeypatch.setattr(DefaultAccountAdapter, "post_login", lambda *_args, **_kwargs: response)

    CustomAccountAdapter().post_login(
        request,
        object(),
        email_verification="none",
        signal_kwargs=None,
        email="existing@example.com",
        signup=False,
        redirect_url="/",
    )

    assert REGISTRATION_COMPLETED_SESSION_KEY not in request.session
