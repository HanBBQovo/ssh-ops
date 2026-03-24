#!/usr/bin/env python3
from __future__ import annotations

import re
import sys
from pathlib import Path


FRONTMATTER_PATTERN = re.compile(r"^---\n(.*?)\n---\n", re.DOTALL)
NAME_PATTERN = re.compile(r"^[a-z0-9][a-z0-9-]{0,63}$")


def parse_frontmatter(text: str) -> dict[str, str]:
    match = FRONTMATTER_PATTERN.match(text)
    if not match:
        raise ValueError("SKILL.md must start with YAML frontmatter delimited by ---")

    values: dict[str, str] = {}
    for raw_line in match.group(1).splitlines():
        line = raw_line.strip()
        if not line:
            continue
        if ":" not in line:
            raise ValueError(f"invalid frontmatter line: {raw_line}")
        key, value = line.split(":", 1)
        values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def validate_skill_dir(skill_dir: Path) -> list[str]:
    errors: list[str] = []
    if not skill_dir.exists():
        return [f"skill directory does not exist: {skill_dir}"]

    skill_md = skill_dir / "SKILL.md"
    if not skill_md.exists():
        return [f"missing file: {skill_md}"]

    text = skill_md.read_text(encoding="utf-8")
    try:
        frontmatter = parse_frontmatter(text)
    except ValueError as exc:
        return [str(exc)]

    name = frontmatter.get("name", "")
    description = frontmatter.get("description", "")
    if not name:
        errors.append("frontmatter field 'name' is required")
    if not description:
        errors.append("frontmatter field 'description' is required")
    if name and not NAME_PATTERN.match(name):
        errors.append("frontmatter field 'name' must use lowercase letters, digits, and hyphens only")
    if name and skill_dir.name != name:
        errors.append(f"skill directory name '{skill_dir.name}' must match frontmatter name '{name}'")

    openai_yaml = skill_dir / "agents" / "openai.yaml"
    if openai_yaml.exists():
        content = openai_yaml.read_text(encoding="utf-8")
        required_tokens = [
            "interface:",
            "display_name:",
            "short_description:",
            "default_prompt:",
        ]
        for token in required_tokens:
            if token not in content:
                errors.append(f"agents/openai.yaml is missing '{token}'")

    return errors


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: validate_skill.py <path/to/skill>", file=sys.stderr)
        return 2

    skill_dir = Path(sys.argv[1]).resolve()
    errors = validate_skill_dir(skill_dir)
    if errors:
        for error in errors:
            print(f"[ERROR] {error}", file=sys.stderr)
        return 1

    print("Skill is valid!")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

