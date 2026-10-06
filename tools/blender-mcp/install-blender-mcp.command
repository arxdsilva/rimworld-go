#!/bin/bash
# Installs the blender-mcp server for the Claude desktop app (macOS).
# Safe to re-run: backs up the config and replaces only the "blender" entry.
set -euo pipefail
export PATH="$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:$HOME/.homebrew/bin:$PATH"

echo "==> Checking for uv..."
if ! command -v uvx >/dev/null 2>&1; then
  if command -v brew >/dev/null 2>&1; then
    brew install uv
  else
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="$HOME/.local/bin:$PATH"
  fi
fi
UVX="$(command -v uvx)"
echo "    uvx: $UVX ($("$UVX" --version))"

echo "==> Pre-fetching blender-mcp..."
"$UVX" blender-mcp --help >/dev/null 2>&1 || true

CFG_DIR="$HOME/Library/Application Support/Claude"
CFG="$CFG_DIR/claude_desktop_config.json"
mkdir -p "$CFG_DIR"
[ -s "$CFG" ] || echo '{}' > "$CFG"
cp "$CFG" "$CFG.bak.$(date +%Y%m%d%H%M%S)"

echo "==> Adding blender server to $CFG"
CFG="$CFG" UVX="$UVX" osascript -l JavaScript <<'JS'
ObjC.import('Foundation');
const env = $.NSProcessInfo.processInfo.environment;
const cfgPath = ObjC.unwrap(env.objectForKey('CFG'));
const uvx = ObjC.unwrap(env.objectForKey('UVX'));
const raw = ObjC.unwrap($.NSString.stringWithContentsOfFileEncodingError(cfgPath, $.NSUTF8StringEncoding, null));
const cfg = JSON.parse(raw || '{}');
cfg.mcpServers = cfg.mcpServers || {};
cfg.mcpServers.blender = { command: uvx, args: ['blender-mcp'], env: { DISABLE_TELEMETRY: 'true' } };
$(JSON.stringify(cfg, null, 2) + '\n').writeToFileAtomicallyEncodingError(cfgPath, true, $.NSUTF8StringEncoding, null);
JS
echo
cat "$CFG"
echo
echo "Done. Quit and reopen the Claude desktop app to load the Blender tools."
