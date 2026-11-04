#!/usr/bin/env python3
"""Validate local documentation links and Draw.io XML files."""

from __future__ import annotations

import re
import sys
import urllib.parse
import xml.etree.ElementTree as ET
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
MARKDOWN_LINK = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
REMOTE_SCHEMES = ("http://", "https://", "mailto:", "tel:")


def validate_markdown_links() -> list[str]:
    """Return errors for local Markdown links whose targets do not exist."""
    errors: list[str] = []

    for document in sorted(ROOT.rglob("*.md")):
        if ".git" in document.parts:
            continue

        text = document.read_text(encoding="utf-8")
        for raw_target in MARKDOWN_LINK.findall(text):
            target = raw_target.strip().split(maxsplit=1)[0].strip("<>")
            if not target or target.startswith("#") or target.startswith(REMOTE_SCHEMES):
                continue

            path_text = urllib.parse.unquote(target.split("#", 1)[0].split("?", 1)[0])
            resolved = (document.parent / path_text).resolve()
            if not resolved.exists():
                relative_document = document.relative_to(ROOT)
                errors.append(f"{relative_document}: missing link target {target!r}")

    return errors


def validate_drawio_xml() -> list[str]:
    """Return errors for Draw.io files that are not well-formed XML."""
    errors: list[str] = []

    for diagram in sorted(ROOT.rglob("*.drawio")):
        try:
            root = ET.parse(diagram).getroot()
        except ET.ParseError as error:
            errors.append(f"{diagram.relative_to(ROOT)}: invalid XML: {error}")
            continue

        if root.tag != "mxfile":
            errors.append(f"{diagram.relative_to(ROOT)}: root element must be mxfile")

    return errors


def main() -> int:
    """Run documentation checks and return a process exit code."""
    errors = validate_markdown_links() + validate_drawio_xml()
    if errors:
        print("Documentation validation failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print("Documentation validation passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
