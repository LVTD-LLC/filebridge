from __future__ import annotations

import re
from dataclasses import dataclass


class FormulaValidationError(ValueError):
    pass


@dataclass(frozen=True)
class FormulaLiteral:
    value: str | int | float | bool


@dataclass(frozen=True)
class FormulaColumn:
    name: str


@dataclass(frozen=True)
class FormulaCall:
    name: str
    arguments: tuple[FormulaExpression, ...]


@dataclass(frozen=True)
class FormulaComparison:
    left: FormulaExpression
    operator: str
    right: FormulaExpression


type FormulaExpression = FormulaLiteral | FormulaColumn | FormulaCall | FormulaComparison

SUPPORTED_FORMULA_FUNCTIONS = {
    "AND",
    "DATEADD",
    "IF",
    "NOT",
    "NOW",
    "OR",
    "SWITCH",
    "TODAY",
}
MAX_FORMULA_LENGTH = 4000
MAX_FORMULA_DEPTH = 32
COMPARISON_OPERATORS = (">=", "<=", "!=", "=", ">", "<")
NUMBER_PATTERN = re.compile(r"-?\d+(?:\.\d+)?")
IDENTIFIER_PATTERN = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


class _FormulaParser:
    def __init__(self, formula: str):
        self.formula = formula
        self.position = 0
        self.depth = 0

    def parse(self) -> FormulaExpression:
        self._skip_whitespace()
        if self._at_end:
            raise FormulaValidationError("Formula cannot be blank.")
        expression = self._parse_expression()
        self._skip_whitespace()
        if not self._at_end:
            raise FormulaValidationError(f"Unexpected character '{self.formula[self.position]}'.")
        return expression

    @property
    def _at_end(self) -> bool:
        return self.position >= len(self.formula)

    def _skip_whitespace(self) -> None:
        while not self._at_end and self.formula[self.position].isspace():
            self.position += 1

    def _parse_expression(self) -> FormulaExpression:
        self.depth += 1
        if self.depth > MAX_FORMULA_DEPTH:
            raise FormulaValidationError(
                f"Formula nesting cannot exceed {MAX_FORMULA_DEPTH} levels."
            )
        try:
            left = self._parse_primary()
            self._skip_whitespace()
            operator = self._match_operator()
            if operator is None:
                return left
            right = self._parse_primary()
            return FormulaComparison(left=left, operator=operator, right=right)
        finally:
            self.depth -= 1

    def _parse_primary(self) -> FormulaExpression:
        self._skip_whitespace()
        if self._at_end:
            raise FormulaValidationError("Formula ended before an expression was complete.")

        character = self.formula[self.position]
        if character == "{":
            return self._parse_column()
        if character in {'"', "'"}:
            return FormulaLiteral(self._parse_string())
        if character == "(":
            self.position += 1
            expression = self._parse_expression()
            self._expect(")")
            return expression

        number_match = NUMBER_PATTERN.match(self.formula, self.position)
        if number_match:
            value = number_match.group(0)
            self.position = number_match.end()
            return FormulaLiteral(float(value) if "." in value else int(value))

        identifier_match = IDENTIFIER_PATTERN.match(self.formula, self.position)
        if identifier_match:
            identifier = identifier_match.group(0).upper()
            self.position = identifier_match.end()
            if identifier in {"TRUE", "FALSE"}:
                return FormulaLiteral(identifier == "TRUE")
            return self._parse_call(identifier)

        raise FormulaValidationError(f"Unexpected character '{character}'.")

    def _parse_column(self) -> FormulaColumn:
        closing_position = self.formula.find("}", self.position + 1)
        if closing_position < 0:
            raise FormulaValidationError("Column reference is missing a closing '}'.")
        name = self.formula[self.position + 1 : closing_position].strip()
        if not name:
            raise FormulaValidationError("Column references cannot be blank.")
        self.position = closing_position + 1
        return FormulaColumn(name=name)

    def _parse_string(self) -> str:
        quote = self.formula[self.position]
        self.position += 1
        characters: list[str] = []
        while not self._at_end:
            character = self.formula[self.position]
            self.position += 1
            if character == quote:
                return "".join(characters)
            if character == "\\":
                if self._at_end:
                    break
                escaped = self.formula[self.position]
                self.position += 1
                characters.append(
                    {
                        "n": "\n",
                        "r": "\r",
                        "t": "\t",
                    }.get(escaped, escaped)
                )
                continue
            characters.append(character)
        raise FormulaValidationError("String literal is missing a closing quote.")

    def _parse_call(self, name: str) -> FormulaCall:
        if name not in SUPPORTED_FORMULA_FUNCTIONS:
            raise FormulaValidationError(f"Unsupported formula function '{name}'.")
        self._expect("(")
        arguments: list[FormulaExpression] = []
        self._skip_whitespace()
        if self._peek(")"):
            self.position += 1
        else:
            while True:
                arguments.append(self._parse_expression())
                self._skip_whitespace()
                if self._peek(")"):
                    self.position += 1
                    break
                self._expect(",")
        self._validate_argument_count(name, arguments)
        return FormulaCall(name=name, arguments=tuple(arguments))

    def _validate_argument_count(
        self,
        name: str,
        arguments: list[FormulaExpression],
    ) -> None:
        exact_counts = {
            "DATEADD": 3,
            "IF": 3,
            "NOT": 1,
            "NOW": 0,
            "TODAY": 0,
        }
        if name in exact_counts and len(arguments) != exact_counts[name]:
            count = exact_counts[name]
            raise FormulaValidationError(
                f"{name} expects {count} argument{'s' if count != 1 else ''}."
            )
        if name in {"AND", "OR"} and not arguments:
            raise FormulaValidationError(f"{name} expects at least 1 argument.")
        if name == "SWITCH" and len(arguments) < 3:
            raise FormulaValidationError("SWITCH expects at least 3 arguments.")

    def _match_operator(self) -> str | None:
        self._skip_whitespace()
        for operator in COMPARISON_OPERATORS:
            if self.formula.startswith(operator, self.position):
                self.position += len(operator)
                return operator
        return None

    def _expect(self, expected: str) -> None:
        self._skip_whitespace()
        if self._peek(expected):
            self.position += len(expected)
            return
        raise FormulaValidationError(f"Expected '{expected}'.")

    def _peek(self, value: str) -> bool:
        return self.formula.startswith(value, self.position)


def parse_formula(formula: str) -> FormulaExpression:
    normalized_formula = str(formula or "").strip()
    if len(normalized_formula) > MAX_FORMULA_LENGTH:
        raise FormulaValidationError(f"Formula must be {MAX_FORMULA_LENGTH} characters or fewer.")
    return _FormulaParser(normalized_formula).parse()


def formula_column_references(expression: FormulaExpression) -> set[str]:
    if isinstance(expression, FormulaColumn):
        return {expression.name}
    if isinstance(expression, FormulaCall):
        references: set[str] = set()
        for argument in expression.arguments:
            references.update(formula_column_references(argument))
        return references
    if isinstance(expression, FormulaComparison):
        return formula_column_references(expression.left) | formula_column_references(
            expression.right
        )
    return set()


def validate_formula_dependencies(
    headers: list[str],
    formulas: dict[str, str],
) -> dict[str, FormulaExpression]:
    header_set = set(headers)
    parsed_formulas = {column: parse_formula(formula) for column, formula in formulas.items()}
    dependencies: dict[str, set[str]] = {}
    for column, expression in parsed_formulas.items():
        references = formula_column_references(expression)
        unknown_columns = sorted(references - header_set)
        if unknown_columns:
            raise FormulaValidationError(
                f"Formula column '{column}' references unknown column '{unknown_columns[0]}'."
            )
        dependencies[column] = references & set(parsed_formulas)

    visiting: list[str] = []
    visited: set[str] = set()

    def visit(column: str) -> None:
        if column in visited:
            return
        if column in visiting:
            cycle_start = visiting.index(column)
            cycle = [*visiting[cycle_start:], column]
            raise FormulaValidationError(
                f"Formula dependency cycle detected: {' -> '.join(cycle)}."
            )
        visiting.append(column)
        for dependency in sorted(dependencies[column]):
            visit(dependency)
        visiting.pop()
        visited.add(column)

    for column in formulas:
        visit(column)
    return parsed_formulas
