import re

INDEXNOW_KEY_PATH = "/indexnow-key.txt"
INDEXNOW_KEY_PATTERN = re.compile(r"^[A-Za-z0-9-]{8,128}$")


def is_valid_indexnow_key(value: str) -> bool:
    return bool(INDEXNOW_KEY_PATTERN.fullmatch(value))
