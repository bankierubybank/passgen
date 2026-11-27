# Security policy

## Reporting a vulnerability

Please report security problems privately through GitHub's
[private vulnerability reporting](https://github.com/bankierubybank/passgen/security/advisories/new)
(Security tab → *Report a vulnerability*). Don't open a public issue.

Helpful to include:

- the passgen version (`passgen -v`) and your OS
- what you did, what happened, and what you expected
- whether it affects the passwords themselves (randomness, character sets,
  length) or something else (update check, clipboard, release files)

You can expect a first reply within a week. Once a fix is released, the
advisory is published with credit to you, unless you'd rather stay anonymous.

## Supported versions

Only the latest release gets fixes. Check yours with `passgen --check-update`.

## What counts

Especially welcome:

- anything that makes generated passwords less random than claimed: bias,
  repeats, a weak random source, wrong strength numbers
- passgen sending anything over the network other than the opt-in
  `--check-update` request
- passwords leaking where they shouldn't (logs, files, other processes)
- release files that don't match their checksum, attestation or SBOM

Out of scope: `passgen check` scoring a weak password too high is a known
limit (its word list is small, see the README), unless the gap is large and
common, like the long repeats that early versions rated strong.
