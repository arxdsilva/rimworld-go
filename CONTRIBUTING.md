# Contributing

Thanks for helping with rimworld-go.

## Sign the CLA first

Every contributor signs the [Contributor License Agreement](CLA.md) once, before their first pull request can be merged.

**Why:** rimworld-go is free under GPL-3.0, and is also sold under commercial licenses to studios that ship closed-source games (see [`COMMERCIAL-LICENSING.md`](COMMERCIAL-LICENSING.md)). The CLA lets the maintainer include your work in both. **You keep the copyright** in what you write. The CLA gives the maintainer a permanent license to use, relicense and sublicense it, plus a patent license.

**How:** open your pull request. A bot comments with a link to the CLA. If you agree, reply on the pull request with exactly:

> I have read the CLA Document and I hereby sign the CLA

The check turns green, and your signature covers all your later pull requests. If the check doesn't update, comment `recheck`.

If your employer has rights in what you write, get their permission before you sign.

## Before you change code

- Read [`AGENTS.md`](AGENTS.md). It sets the rules: hexagonal architecture, and a headless, deterministic domain.
- Build, test and profile as [`docs/DEVELOPING.md`](docs/DEVELOPING.md) describes. Tests run with `-race`.
- New Go files start with the license header; `python3 scripts/spdx_headers.py` adds it.
- Don't copy anything from RimWorld or any other proprietary game: no code, art, text or data ([`.agents/specs/CLEAN_ROOM.md`](.agents/specs/CLEAN_ROOM.md)).
