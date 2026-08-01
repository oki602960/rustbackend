#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

WRITE_STUBS=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --write-stubs)
      WRITE_STUBS=1
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

export ROOT_DIR
export WRITE_STUBS

uv run python - <<'PY'
import ast
import json
import os
import re
import subprocess
from collections import defaultdict
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(os.environ["ROOT_DIR"])
WRITE_STUBS = os.environ["WRITE_STUBS"] == "1"
PY_ROOT = ROOT / "open-webui" / "backend" / "open_webui"
BACKEND_ROOT = ROOT / "backend"
GENERATED_DIR = BACKEND_ROOT / "_generated"
SYNC_MAP_PATH = BACKEND_ROOT / "SYNC_MAP.yaml"


def yaml_quote(value: str) -> str:
    return json.dumps(value, ensure_ascii=False)


def go_path_for(py_path: Path) -> Path:
    rel = py_path.relative_to(PY_ROOT)
    parts = list(rel.parts)
    name = parts[-1]
    if name == "__init__.py":
        parts[-1] = "init__.go"
    elif rel.parts and rel.parts[0] == "test" and name.endswith(".py"):
        parts[-1] = name[:-3] + "_test.go"
    else:
        parts[-1] = name[:-3] + ".go"
    return Path("backend") / "open_webui" / Path(*parts)


def package_name_for(go_path: Path) -> str:
    rel = go_path.relative_to(BACKEND_ROOT)
    parent = rel.parent
    if str(parent) == "open_webui":
        return "openwebui"
    return parent.name


def load_existing_sync_map() -> dict[str, dict]:
    if not SYNC_MAP_PATH.exists():
        return {}

    existing: dict[str, dict] = {}
    current: dict | None = None
    for raw_line in SYNC_MAP_PATH.read_text(encoding="utf-8").splitlines():
        line = raw_line.rstrip()
        if line.startswith("  - python_path: "):
            if current and current.get("python_path"):
                existing[current["python_path"]] = current
            current = {
                "python_path": json.loads(line.split(": ", 1)[1]),
                "go_path": "",
                "status": "stub",
                "companions": [],
                "source_submodule_sha": "",
                "last_verified_at": None,
            }
            continue
        if current is None:
            continue
        if line.startswith("    go_path: "):
            current["go_path"] = json.loads(line.split(": ", 1)[1])
        elif line.startswith("    status: "):
            current["status"] = json.loads(line.split(": ", 1)[1])
        elif line.startswith("    companions: "):
            payload = line.split(": ", 1)[1].strip()
            current["companions"] = json.loads(payload) if payload != "[]" else []
        elif line.startswith("    source_submodule_sha: "):
            current["source_submodule_sha"] = json.loads(line.split(": ", 1)[1])
        elif line.startswith("    last_verified_at: "):
            payload = line.split(": ", 1)[1].strip()
            current["last_verified_at"] = None if payload == "null" else json.loads(payload)
    if current and current.get("python_path"):
        existing[current["python_path"]] = current
    return existing


def extract_routes(py_path: Path) -> list[dict]:
    verbs = {"get", "post", "put", "delete", "patch", "options", "head", "api_route"}
    tree = ast.parse(py_path.read_text(encoding="utf-8"))
    routes = []
    for node in tree.body:
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            continue
        for dec in node.decorator_list:
            if not isinstance(dec, ast.Call):
                continue
            func = dec.func
            if not isinstance(func, ast.Attribute):
                continue
            if not isinstance(func.value, ast.Name) or func.value.id != "router":
                continue
            if func.attr not in verbs:
                continue

            path = None
            methods = None
            if dec.args and isinstance(dec.args[0], ast.Constant):
                path = dec.args[0].value
            if func.attr == "api_route":
                for kw in dec.keywords:
                    if kw.arg == "methods" and isinstance(kw.value, (ast.List, ast.Tuple)):
                        methods = []
                        for elt in kw.value.elts:
                            if isinstance(elt, ast.Constant):
                                methods.append(str(elt.value))
            routes.append(
                {
                    "file": str(py_path.relative_to(ROOT).as_posix()),
                    "handler": node.name,
                    "decorator": func.attr,
                    "path": path,
                    "methods": methods,
                }
            )
    return routes


def extract_env_keys() -> list[str]:
    env_text = (PY_ROOT / "env.py").read_text(encoding="utf-8")
    patterns = [
        r'os\.getenv\(\s*[\'"]([^\'"]+)[\'"]',
        r'os\.environ\.get\(\s*[\'"]([^\'"]+)[\'"]',
        r'os\.environ\[\s*[\'"]([^\'"]+)[\'"]\s*\]',
    ]
    keys = set()
    for pattern in patterns:
        keys.update(re.findall(pattern, env_text))
    return sorted(keys)


def extract_persistent_config_keys() -> list[dict]:
    config_text = (PY_ROOT / "config.py").read_text(encoding="utf-8")
    pattern = re.compile(
        r"PersistentConfig\(\s*[\'\"]([^\'\"]+)[\'\"]\s*,\s*[\'\"]([^\'\"]+)[\'\"]",
        re.MULTILINE,
    )
    return [{"env_name": env_name, "config_path": config_path} for env_name, config_path in pattern.findall(config_text)]


def write_json(path: Path, payload) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=False) + "\n", encoding="utf-8")


def write_lines(path: Path, lines: list[str]) -> None:
    path.write_text("".join(f"{line}\n" for line in lines), encoding="utf-8")


def maybe_write_stub(go_path: Path) -> None:
    abs_path = ROOT / go_path
    abs_path.parent.mkdir(parents=True, exist_ok=True)
    pkg = package_name_for(abs_path)
    if not abs_path.exists():
        abs_path.write_text(f"package {pkg}\n", encoding="utf-8")
        return
    content = abs_path.read_text(encoding="utf-8")
    if content.strip() == "":
        abs_path.write_text(f"package {pkg}\n", encoding="utf-8")


GENERATED_DIR.mkdir(parents=True, exist_ok=True)
existing = load_existing_sync_map()
py_files = sorted(PY_ROOT.rglob("*.py"))

submodule_sha = (
    subprocess.run(
        ["git", "-C", str(ROOT), "submodule", "status", "--", "open-webui"],
        capture_output=True,
        text=True,
        check=True,
    )
    .stdout.strip()
    .split()[0]
    .lstrip("-+")
)

entries = []
dir_counts: dict[str, dict[str, int]] = defaultdict(lambda: {"files": 0, "lines": 0})
routes: list[dict] = []
all_go_files: list[str] = []
for py_file in py_files:
    rel = py_file.relative_to(ROOT).as_posix()
    go_path = go_path_for(py_file).as_posix()
    all_go_files.append(go_path)

    preserved = existing.get(rel, {})
    entries.append(
        {
            "python_path": rel,
            "go_path": go_path,
            "status": preserved.get("status", "stub"),
            "companions": preserved.get("companions", []),
            "source_submodule_sha": submodule_sha,
            "last_verified_at": preserved.get("last_verified_at"),
        }
    )

    top = py_file.relative_to(PY_ROOT).parts[0] if len(py_file.relative_to(PY_ROOT).parts) > 1 else "root"
    line_count = sum(1 for _ in py_file.open("r", encoding="utf-8"))
    dir_counts[top]["files"] += 1
    dir_counts[top]["lines"] += line_count

    if py_file.parts[-2] == "routers":
        routes.extend(extract_routes(py_file))

    if WRITE_STUBS:
        maybe_write_stub(Path(go_path))

sync_map_lines = [
    "schema_version: 1",
    f"generated_at: {yaml_quote(datetime.now(timezone.utc).isoformat())}",
    f"source_submodule_sha: {yaml_quote(submodule_sha)}",
    "entries:",
]
for entry in entries:
    companions = json.dumps(entry["companions"], ensure_ascii=False)
    last_verified_at = "null" if entry["last_verified_at"] is None else yaml_quote(entry["last_verified_at"])
    sync_map_lines.extend(
        [
            f"  - python_path: {yaml_quote(entry['python_path'])}",
            f"    go_path: {yaml_quote(entry['go_path'])}",
            f"    status: {yaml_quote(entry['status'])}",
            f"    companions: {companions}",
            f"    source_submodule_sha: {yaml_quote(entry['source_submodule_sha'])}",
            f"    last_verified_at: {last_verified_at}",
        ]
    )
SYNC_MAP_PATH.write_text("\n".join(sync_map_lines) + "\n", encoding="utf-8")

write_lines(GENERATED_DIR / "python_files.txt", [entry["python_path"] for entry in entries])
write_lines(GENERATED_DIR / "go_files.txt", all_go_files)
write_json(GENERATED_DIR / "python_routes.json", routes)
write_lines(GENERATED_DIR / "python_env_keys.txt", extract_env_keys())
write_json(GENERATED_DIR / "python_persistent_config_keys.json", extract_persistent_config_keys())
write_lines(
    GENERATED_DIR / "python_migrations.txt",
    sorted(
        str(path.relative_to(ROOT).as_posix())
        for path in [
            *PY_ROOT.joinpath("internal", "migrations").glob("*.py"),
            *PY_ROOT.joinpath("migrations", "versions").glob("*.py"),
        ]
    ),
)
write_json(
    GENERATED_DIR / "python_stats.json",
    {
        "python_file_count": len(entries),
        "directory_stats": {key: dir_counts[key] for key in sorted(dir_counts)},
        "http_route_count": len(routes),
    },
)
PY
