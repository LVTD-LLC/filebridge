import csv
import io

import pytest

from apps.datasets.choices import DatasetColumnType
from apps.datasets.models import Dataset, DatasetRow

pytestmark = pytest.mark.django_db


def create_personal_crm_dataset(profile):
    dataset = Dataset.objects.create(
        profile=profile,
        name="Personal CRM",
        headers=["person_id", "name", "category", "last_contact"],
        column_schema={
            "person_id": {"type": DatasetColumnType.TEXT},
            "name": {"type": DatasetColumnType.TEXT},
            "category": {
                "type": DatasetColumnType.CHOICE,
                "choices": ["A", "B", "C", "D"],
            },
            "last_contact": {"type": DatasetColumnType.DATE},
        },
        index_column="person_id",
        row_count=2,
    )
    DatasetRow.objects.bulk_create(
        [
            DatasetRow(
                dataset=dataset,
                row_number=1,
                index_value="P-1",
                data={
                    "person_id": "P-1",
                    "name": "Ada Lovelace",
                    "category": "A",
                    "last_contact": "2000-01-01",
                },
            ),
            DatasetRow(
                dataset=dataset,
                row_number=2,
                index_value="P-2",
                data={
                    "person_id": "P-2",
                    "name": "Grace Hopper",
                    "category": "B",
                    "last_contact": "2099-12-01",
                },
            ),
        ]
    )
    return dataset


NEXT_CONTACT_FORMULA = """
SWITCH(
    {category},
    "A", DATEADD({last_contact}, 3, "weeks"),
    "B", DATEADD({last_contact}, 2, "months"),
    "C", DATEADD({last_contact}, 6, "months"),
    "D", DATEADD({last_contact}, 12, "months")
)
""".strip()


def test_formula_columns_power_personal_crm_reads_filters_and_exports(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    next_contact_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "next_contact",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "date",
                "formula": NEXT_CONTACT_FORMULA,
            },
        },
        content_type="application/json",
    )

    assert next_contact_response.status_code == 200
    due_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "trigger_reminder",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "boolean",
                "formula": "AND({next_contact}, TODAY() >= {next_contact})",
            },
        },
        content_type="application/json",
    )
    assert due_response.status_code == 200

    rows_response = api_client.get(
        f"/api/datasets/{dataset.key}/rows",
        {"filters": '{"trigger_reminder":"true"}', "sort": "next_contact"},
    )

    assert rows_response.status_code == 200
    rows = rows_response.json()["rows"]
    assert [row["index_value"] for row in rows] == ["P-1"]
    assert rows[0]["data"]["next_contact"] == "2000-01-22"
    assert rows[0]["data"]["trigger_reminder"] == "true"
    dataset.refresh_from_db()
    assert dataset.rows.filter(data__has_key="next_contact").exists() is False
    assert dataset.rows.filter(data__has_key="trigger_reminder").exists() is False

    export_response = api_client.get(f"/api/datasets/{dataset.key}/export.csv")

    assert export_response.status_code == 200
    exported_rows = list(csv.DictReader(io.StringIO(export_response.content.decode())))
    assert exported_rows[0]["next_contact"] == "2000-01-22"
    assert exported_rows[0]["trigger_reminder"] == "true"
    assert exported_rows[1]["next_contact"] == "2100-02-01"
    assert exported_rows[1]["trigger_reminder"] == "false"


def test_formula_column_rejects_unknown_columns_and_cycles(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    unknown_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "bad_formula",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "date",
                "formula": 'DATEADD({missing}, 1, "day")',
            },
        },
        content_type="application/json",
    )

    assert unknown_response.status_code == 400
    assert "unknown column 'missing'" in unknown_response.json()["detail"].lower()
    assert "bad_formula" not in dataset.headers

    first_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "first_formula",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "text",
                "formula": 'IF({name} = "Ada Lovelace", "yes", "no")',
            },
        },
        content_type="application/json",
    )
    assert first_response.status_code == 200

    cycle_response = api_client.patch(
        f"/api/datasets/{dataset.key}/column-types",
        data={
            "column_types": {
                "first_formula": {
                    "type": "calculated",
                    "calculation": "formula",
                    "result_type": "text",
                    "formula": "{first_formula}",
                }
            }
        },
        content_type="application/json",
    )

    assert cycle_response.status_code == 400
    assert "cycle" in cycle_response.json()["detail"].lower()


def test_formula_column_rejects_invalid_function_semantics(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "bad_date",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "date",
                "formula": 'DATEADD({last_contact}, 1, "fortnight")',
            },
        },
        content_type="application/json",
    )

    assert response.status_code == 400
    assert "DATEADD unit must be day, week, month, or year" in response.json()["detail"]
    dataset.refresh_from_db()
    assert "bad_date" not in dataset.headers


def test_formula_dependencies_block_source_column_rename_and_drop(api_client, profile):
    dataset = create_personal_crm_dataset(profile)
    add_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "next_contact",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "date",
                "formula": 'DATEADD({last_contact}, 3, "weeks")',
            },
        },
        content_type="application/json",
    )
    assert add_response.status_code == 200

    rename_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns/rename",
        data={"old_name": "last_contact", "new_name": "last_interaction"},
        content_type="application/json",
    )
    drop_response = api_client.post(
        f"/api/datasets/{dataset.key}/columns/drop",
        data={"name": "last_contact"},
        content_type="application/json",
    )

    assert rename_response.status_code == 409
    assert "referenced by formula column 'next_contact'" in rename_response.json()["detail"]
    assert drop_response.status_code == 409
    assert "referenced by formula column 'next_contact'" in drop_response.json()["detail"]
