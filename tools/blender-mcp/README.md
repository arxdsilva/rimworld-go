# Blender MCP

Lets an AI agent (Claude Code, Cursor, the Claude desktop app) drive a live
Blender over [MCP for Blender](https://mcp-for-blender.com/). Used for working
on the art in `assets/source/blender`; `make art` does not need it.

Two halves: the **addon** runs inside Blender and listens on `localhost:9876`;
the **server** (`uvx blender-mcp`) is started by the agent and talks to it.

## New machine setup

1. Install [uv](https://docs.astral.sh/uv/) (`brew install uv`), which provides `uvx`.
2. Install the addon in Blender: Edit > Preferences > Add-ons > Install from
   disk > `tools/blender-mcp/addon.py`, then enable "MCP for Blender". It
   starts its server automatically; the sidebar (N) > MCP for Blender shows
   "Connected on port 9876".
3. Point your agent at the server:
   - **Claude Code**: nothing to do. `.mcp.json` at the repo root registers
     `blender`; approve it when Claude Code asks, then check with `/mcp`.
   - **Cursor**: nothing to do. `.cursor/mcp.json` registers it; enable it
     under Settings > MCP.
   - **Claude desktop app** (macOS): run `install-blender-mcp.command`, then
     restart the app.

Keep Blender open while the agent works; the server only connects to a
running Blender.

## Updating the addon

The addon's protocol version must match the server's. If the agent reports
a mismatch, run `uvx mcp-for-blender install-addon` (or use the addon's own
"Update addon" button), copy the new `addon.py` over the one here, and commit
it so every setup gets the same version.
