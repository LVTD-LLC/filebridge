import csv
import io

import pytest
from django.urls import reverse

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
    search_response = api_client.get(
        f"/api/datasets/{dataset.key}/rows",
        {"query": "2000-01-22"},
    )
    assert search_response.status_code == 200
    assert [row["index_value"] for row in search_response.json()["rows"]] == ["P-1"]
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


def test_dataset_creation_accepts_formula_columns_without_storing_formula_values(
    api_client,
    profile,
):
    response = api_client.post(
        "/api/datasets",
        data={
            "name": "Formula CRM",
            "headers": ["person_id", "last_contact", "next_contact"],
            "index_column": "person_id",
            "column_types": {
                "last_contact": "date",
                "next_contact": {
                    "type": "calculated",
                    "calculation": "formula",
                    "result_type": "date",
                    "formula": 'DATEADD({last_contact}, 3, "weeks")',
                },
            },
            "rows": [
                {
                    "person_id": "P-1",
                    "last_contact": "2000-01-01",
                    "next_contact": "do not store me",
                }
            ],
        },
        content_type="application/json",
    )

    assert response.status_code == 201
    dataset = Dataset.objects.get(key=response.json()["dataset"]["key"])
    row = dataset.rows.get()
    assert "next_contact" not in row.data

    rows_response = api_client.get(f"/api/datasets/{dataset.key}/rows")

    assert rows_response.status_code == 200
    assert rows_response.json()["rows"][0]["data"]["next_contact"] == "2000-01-22"


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


def test_formula_casts_invalid_row_values_to_empty_without_breaking_reads(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "name_as_number",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "number",
                "formula": "{name}",
            },
        },
        content_type="application/json",
    )

    assert response.status_code == 200
    rows_response = api_client.get(f"/api/datasets/{dataset.key}/rows")
    assert rows_response.status_code == 200
    assert [row["data"]["name_as_number"] for row in rows_response.json()["rows"]] == ["", ""]


def test_formula_if_coerces_mixed_branches_without_invalid_sql(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "mixed_label",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "text",
                "formula": 'IF({category} = "A", 1, "unknown")',
            },
        },
        content_type="application/json",
    )

    assert response.status_code == 200
    rows_response = api_client.get(f"/api/datasets/{dataset.key}/rows")
    assert rows_response.status_code == 200
    assert [row["data"]["mixed_label"] for row in rows_response.json()["rows"]] == [
        "1",
        "unknown",
    ]


def test_formula_inputs_use_supported_date_and_currency_formats(api_client, profile):
    dataset = create_personal_crm_dataset(profile)
    dataset.headers.append("budget")
    dataset.column_schema["budget"] = {"type": DatasetColumnType.CURRENCY}
    dataset.save(update_fields=["headers", "column_schema"])
    first_row, second_row = dataset.rows.order_by("row_number")
    first_row.data.update({"last_contact": "01/02/2000", "budget": "$1,200.50"})
    second_row.data.update({"last_contact": "2099/12/01", "budget": "€2,500"})
    DatasetRow.objects.bulk_update([first_row, second_row], ["data"])

    for name, result_type, formula in [
        ("next_contact", "date", 'DATEADD({last_contact}, 1, "day")'),
        ("normalized_budget", "currency", "{budget}"),
    ]:
        response = api_client.post(
            f"/api/datasets/{dataset.key}/columns",
            data={
                "name": name,
                "column_type": {
                    "type": "calculated",
                    "calculation": "formula",
                    "result_type": result_type,
                    "formula": formula,
                },
            },
            content_type="application/json",
        )
        assert response.status_code == 200

    rows_response = api_client.get(f"/api/datasets/{dataset.key}/rows")
    assert rows_response.status_code == 200
    rows = rows_response.json()["rows"]
    assert rows[0]["data"]["next_contact"] == "2000-01-03"
    assert rows[0]["data"]["normalized_budget"] == "1200.5"
    assert rows[1]["data"]["next_contact"] == "2099-12-02"
    assert rows[1]["data"]["normalized_budget"] == "2500.0"


def test_formula_comparison_coerces_text_literal_to_typed_operand(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "contacted_on_new_year",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "boolean",
                "formula": '{last_contact} = "2000-01-01"',
            },
        },
        content_type="application/json",
    )

    assert response.status_code == 200
    rows_response = api_client.get(f"/api/datasets/{dataset.key}/rows")
    assert rows_response.status_code == 200
    assert [row["data"]["contacted_on_new_year"] for row in rows_response.json()["rows"]] == [
        "true",
        "false",
    ]


def test_formula_compiler_supports_if_or_not_and_now(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "needs_attention",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "text",
                "formula": ('IF(OR(NOT({category} = "A"), NOW() >= {last_contact}), "yes", "no")'),
            },
        },
        content_type="application/json",
    )

    assert response.status_code == 200
    rows_response = api_client.get(f"/api/datasets/{dataset.key}/rows")
    assert rows_response.status_code == 200
    assert [row["data"]["needs_attention"] for row in rows_response.json()["rows"]] == [
        "yes",
        "yes",
    ]


def test_formula_rejects_incompatible_comparison_types(api_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = api_client.post(
        f"/api/datasets/{dataset.key}/columns",
        data={
            "name": "invalid_comparison",
            "column_type": {
                "type": "calculated",
                "calculation": "formula",
                "result_type": "boolean",
                "formula": "{last_contact} = TRUE",
            },
        },
        content_type="application/json",
    )

    assert response.status_code == 400
    assert "Cannot compare date and boolean values" in response.json()["detail"]


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


def test_dataset_settings_adds_formula_column_through_htmx(auth_client, profile):
    dataset = create_personal_crm_dataset(profile)

    settings_response = auth_client.get(reverse("dataset_settings", args=[dataset.key]))

    assert settings_response.status_code == 200
    settings_content = settings_response.content.decode()
    assert 'id="formula-columns-panel"' in settings_content
    assert f'hx-post="{reverse("dataset_upsert_formula_column", args=[dataset.key])}"' in (
        settings_content
    )
    assert 'hx-target="#formula-columns-panel"' in settings_content

    response = auth_client.post(
        reverse("dataset_upsert_formula_column", args=[dataset.key]),
        {
            "name": "next_contact",
            "result_type": "date",
            "formula": 'DATEADD({last_contact}, 3, "weeks")',
        },
        HTTP_HX_REQUEST="true",
    )

    assert response.status_code == 200
    assert "HX-Request" in response.headers["Vary"]
    content = response.content.decode()
    assert '<section id="formula-columns-panel"' in content
    assert "<html" not in content
    assert "Formula column added." in content
    assert "next_contact" in content
    dataset.refresh_from_db()
    assert dataset.column_schema["next_contact"] == {
        "type": "calculated",
        "calculation": "formula",
        "result_type": "date",
        "formula": 'DATEADD({last_contact}, 3, "weeks")',
    }

    detail_response = auth_client.get(dataset.get_absolute_url())
    next_contact_filter = next(
        field
        for field in detail_response.context["row_filter_fields"]
        if field["header"] == "next_contact"
    )
    assert next_contact_filter["input_type"] == "date"
    assert next_contact_filter["is_ordered_filter"] is True
    assert next_contact_filter["sort_ascending_label"] == "Oldest first"


def test_dataset_settings_adds_formula_column_without_htmx(auth_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = auth_client.post(
        reverse("dataset_upsert_formula_column", args=[dataset.key]),
        {
            "name": "next_contact",
            "result_type": "date",
            "formula": 'DATEADD({last_contact}, 3, "weeks")',
        },
    )

    assert response.status_code == 302
    assert response.url == reverse("dataset_settings", args=[dataset.key])
    dataset.refresh_from_db()
    assert dataset.column_schema["next_contact"]["calculation"] == "formula"


def test_dataset_settings_formula_htmx_error_preserves_form_values(auth_client, profile):
    dataset = create_personal_crm_dataset(profile)

    response = auth_client.post(
        reverse("dataset_upsert_formula_column", args=[dataset.key]),
        {
            "name": "next_contact",
            "result_type": "date",
            "formula": 'DATEADD({missing}, 3, "weeks")',
        },
        HTTP_HX_REQUEST="true",
    )

    assert response.status_code == 200
    content = response.content.decode()
    assert "references unknown column" in content
    assert 'value="next_contact"' in content
    assert "DATEADD({missing}, 3, &quot;weeks&quot;)" in content
    dataset.refresh_from_db()
    assert "next_contact" not in dataset.headers
