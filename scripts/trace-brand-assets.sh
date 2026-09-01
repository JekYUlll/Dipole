#!/usr/bin/env bash
set -euo pipefail

# Recreate the V3 lockups from the approved PNG without hand-drawing paths.
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_png="$root_dir/docs/images/LOGO_V3.png"
work_dir="${TMPDIR:-/tmp}/dipole-brand-trace"

command -v magick >/dev/null || { echo "magick is required" >&2; exit 1; }
command -v vtracer >/dev/null || { echo "vtracer is required" >&2; exit 1; }
test -f "$source_png" || { echo "missing $source_png" >&2; exit 1; }

mkdir -p "$work_dir"

trace() {
  local crop="$1"
  local output="$2"
  magick "$source_png" -crop "$crop" +repage "$work_dir/input.png"
  vtracer --input "$work_dir/input.png" --output "$root_dir/docs/images/$output" \
    --preset poster --clustering color-cluster --hierarchical stacked \
    --mode spline --color-precision 6 --gradient-step 24 \
    --filter-speckle 16 --simplify 2.2 --path-precision 2 --optimize 2
}

# Crops are measured against LOGO_V3.png (1448x1086).
trace '610x560+0+270' 'dipole-v3-im-traced.svg'
trace '680x650+720+150' 'dipole-v3-agent-traced.svg'
trace '680x540+720+150' 'dipole-v3-agent-mark-traced.svg'

echo "Traced V3 brand assets into docs/images/"
