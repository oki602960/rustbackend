#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PY_ROUTES="${1:-$ROOT_DIR/backend/_generated/python_routes.json}"
GO_ROUTES="${2:-$ROOT_DIR/backend/_generated/go_routes.json}"

uv run python - "$PY_ROUTES" "$GO_ROUTES" <<'PY'
import json
import sys
from difflib import unified_diff
from pathlib import Path

left = Path(sys.argv[1])
right = Path(sys.argv[2])

if not left.exists():
    raise SystemExit(f"missing file: {left}")
if not right.exists():
    raise SystemExit(f"missing file: {right}")

left_text = json.dumps(json.loads(left.read_text(encoding="utf-8")), ensure_ascii=False, indent=2, sort_keys=True).splitlines()
right_text = json.dumps(json.loads(right.read_text(encoding="utf-8")), ensure_ascii=False, indent=2, sort_keys=True).splitlines()
diff = list(unified_diff(left_text, right_text, fromfile=str(left), tofile=str(right), lineterm=""))
if diff:
    print("\n".join(diff))
    raise SystemExit(1)
PY
