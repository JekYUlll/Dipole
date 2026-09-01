#!/usr/bin/env bash
set -euo pipefail

# Recreate the V3 lockups from the approved PNG without hand-drawing paths.
# The source canvas is removed before tracing so the SVG remains transparent
# while preserving the raster artwork's actual contours and color treatment.
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
  # Quantize the source and use a binary alpha mask to prevent raster edge
  # noise from becoming visible vector paths.
  magick "$source_png" -crop "$crop" +repage \
    -colorspace sRGB -colors 4 -dither None "$work_dir/color.png"
  magick "$source_png" -crop "$crop" +repage \
    -fuzz 8% -transparent '#FAF1E3' -alpha extract -threshold 50% \
    "$work_dir/mask.png"
  magick "$work_dir/color.png" "$work_dir/mask.png" -alpha off \
    -compose CopyOpacity -composite "$work_dir/input.png"
  vtracer --input "$work_dir/input.png" --output "$root_dir/docs/images/$output" \
    --preset poster --clustering color-cluster --hierarchical stacked \
    --mode spline --color-precision 8 --gradient-step 32 \
    --filter-speckle 16 --simplify 2.0 --path-precision 3 --optimize 2 \
    --max-colors 4
}

# Crops are measured against LOGO_V3.png (1448x1086). The bounds include the
# complete visible lockup/mark and intentionally exclude the concept heading
# and palette swatches.
trace '560x490+100+295' 'dipole-v3-im-traced.svg'
trace '560x360+100+295' 'dipole-v3-im-mark-traced.svg'
trace '680x625+720+175' 'dipole-v3-agent-traced.svg'
trace '680x510+720+175' 'dipole-v3-agent-mark-traced.svg'

echo "Traced V3 brand assets into docs/images/"
