import csv
import io
import json

import polars as pl
from hypothesis import given
from hypothesis import strategies as st

from apps.datasets.choices import DatasetColumnType
from apps.datasets.services import (
    generated_index_column_name,
    normalize_column_schema,
    rows_to_csv_text,
    rows_to_jsonl_text,
    rows_to_parquet_bytes,
    rows_to_sqlite_bytes,
)
from apps.datasets.tests.dataset_test_helpers import sqlite_rows

header_text = (
    st.text(
        alphabet=st.characters(blacklist_categories=("Cc", "Cs")),
        min_size=1,
        max_size=30,
    )
    .map(str.strip)
    .filter(bool)
)
cell_value = st.one_of(
    st.none(),
    st.booleans(),
    st.integers(),
    st.floats(allow_nan=False, allow_infinity=False),
    st.text(
        alphabet=st.characters(blacklist_categories=("Cs",), blacklist_characters="\x00"),
        max_size=100,
    ),
)
simple_column_type = st.sampled_from(
    [
        DatasetColumnType.TEXT,
        DatasetColumnType.TAGS,
        DatasetColumnType.INTEGER,
        DatasetColumnType.NUMBER,
        DatasetColumnType.CURRENCY,
        DatasetColumnType.BOOLEAN,
        DatasetColumnType.DATE,
        DatasetColumnType.DATETIME,
        DatasetColumnType.EMAIL,
        DatasetColumnType.URL,
    ]
)


@st.composite
def valid_tables(draw):
    headers = draw(st.lists(header_text, min_size=1, max_size=8, unique=True))
    row_strategy = st.dictionaries(
        keys=st.sampled_from(headers),
        values=cell_value,
        max_size=len(headers),
    )
    rows = draw(st.lists(row_strategy, max_size=20))
    return headers, rows


@st.composite
def simple_column_schemas(draw):
    headers = draw(st.lists(header_text, min_size=1, max_size=12, unique=True))
    column_types = draw(
        st.lists(
            simple_column_type,
            min_size=len(headers),
            max_size=len(headers),
        )
    )
    return headers, {
        header: {"type": column_type}
        for header, column_type in zip(headers, column_types, strict=True)
    }


def normalized_rows(headers, rows):
    return [
        {header: "" if row.get(header) is None else str(row.get(header, "")) for header in headers}
        for row in rows
    ]


@given(valid_tables())
def test_tabular_exports_round_trip_to_the_same_rows(table):
    headers, rows = table
    expected = normalized_rows(headers, rows)

    csv_export = list(csv.DictReader(io.StringIO(rows_to_csv_text(headers, rows))))
    jsonl_export = [json.loads(line) for line in io.StringIO(rows_to_jsonl_text(headers, rows))]
    parquet_export = pl.read_parquet(io.BytesIO(rows_to_parquet_bytes(headers, rows))).to_dicts()
    sqlite_export = sqlite_rows(rows_to_sqlite_bytes(headers, rows))

    assert csv_export == expected
    assert jsonl_export == expected
    assert parquet_export == expected
    assert sqlite_export == expected


@given(simple_column_schemas())
def test_column_schema_normalization_is_idempotent(schema):
    headers, raw_schema = schema

    normalized = normalize_column_schema(headers, raw_schema)

    assert list(normalized) == headers
    assert normalize_column_schema(headers, normalized) == normalized


@given(st.lists(header_text, unique=True, max_size=30))
def test_generated_index_column_never_collides_with_existing_headers(headers):
    generated_name = generated_index_column_name(headers)

    assert generated_name not in headers
