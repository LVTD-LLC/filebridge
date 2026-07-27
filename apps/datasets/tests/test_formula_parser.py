import re

import pytest

from apps.datasets.formulas import (
    FormulaCall,
    FormulaComparison,
    FormulaValidationError,
    formula_column_references,
    parse_formula,
)


def test_parse_formula_builds_calls_comparisons_and_column_references():
    expression = parse_formula(
        'AND({next_contact}, TODAY() >= DATEADD({last_contact}, 3, "weeks"))'
    )

    assert isinstance(expression, FormulaCall)
    assert expression.name == "AND"
    assert isinstance(expression.arguments[1], FormulaComparison)
    assert formula_column_references(expression) == {"last_contact", "next_contact"}


@pytest.mark.parametrize(
    ("formula", "message"),
    [
        ("", "Formula cannot be blank"),
        ("UNKNOWN({name})", "Unsupported formula function 'UNKNOWN'"),
        ("DATEADD({last_contact}, 1)", "DATEADD expects 3 arguments"),
        ("{name} + 1", "Unexpected character '+'"),
        ("IF({name}", "Expected ','"),
    ],
)
def test_parse_formula_rejects_invalid_or_unsupported_expressions(formula, message):
    with pytest.raises(FormulaValidationError, match=re.escape(message)):
        parse_formula(formula)
