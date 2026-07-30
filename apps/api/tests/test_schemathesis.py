import pytest
import schemathesis
from hypothesis import HealthCheck, settings

from apps.datasets.tests.factories import create_profile_with_api_key
from rowset.wsgi import application

pytestmark = pytest.mark.django_db

schema = schemathesis.openapi.from_wsgi(
    "/api/openapi.json",
    application,
    headers={"Host": "testserver"},
)
read_schema = schema.include(method="GET")
dataset_creation_schema = schema.include(method="POST", path="/api/datasets")


@pytest.fixture
def fuzzing_api_key(django_user_model):
    profile = create_profile_with_api_key(
        django_user_model,
        username="schemathesis",
        email="schemathesis@example.com",
    )
    return profile.key


@read_schema.parametrize()
@settings(
    max_examples=10,
    deadline=None,
    suppress_health_check=[HealthCheck.function_scoped_fixture],
)
def test_openapi_reads_never_return_server_errors(case, fuzzing_api_key):
    case.call_and_validate(
        headers={"Authorization": f"Bearer {fuzzing_api_key}"},
        checks=[schemathesis.checks.not_a_server_error],
    )


@dataset_creation_schema.parametrize()
@settings(
    max_examples=20,
    deadline=None,
    suppress_health_check=[HealthCheck.function_scoped_fixture],
)
def test_dataset_creation_never_returns_server_errors(case, fuzzing_api_key):
    case.call_and_validate(
        headers={"Authorization": f"Bearer {fuzzing_api_key}"},
        checks=[schemathesis.checks.not_a_server_error],
    )
