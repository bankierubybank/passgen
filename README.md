# passgen

[![CI](https://github.com/bankierubybank/passgen/actions/workflows/ci.yml/badge.svg)](https://github.com/bankierubybank/passgen/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/bankierubybank/passgen/badge)](https://scorecard.dev/viewer/?uri=github.com/bankierubybank/passgen)

Generate secure random passwords locally. A single binary with no dependencies that works fully offline. The only network call is an update check, and it runs only when you ask for it.

- Randomness comes from the OS's cryptographic random source (Go `crypto/rand`), with unbiased selection
- Complexity presets, plus per-character-set control
- Three symbol sets, from "accepted everywhere" to "every printable symbol"
- Can exclude look-alike characters (`Il1O0o`…) or any characters you list
- Shows estimated entropy and a strength label
- Clipboard copy on macOS, Windows and Linux
- Interactive mode
- `passgen check`: rates a password you already have, offline
- Shell completion (bash, zsh, fish) and a man page

```
$ passgen
q7#Ndb%Rk2-xT+aW

[73 chars pool, ~99 bits entropy: very strong]
```

## Install

### Download a binary

Get the archive for your system from the [Releases](../../releases) page, extract it, and put `passgen` (or `passgen.exe`) somewhere on your `PATH`.

| System | Archive |
|---|---|
| macOS Apple Silicon | `passgen_<version>_darwin_arm64.tar.gz` |
| macOS Intel | `passgen_<version>_darwin_amd64.tar.gz` |
| Linux x86-64 | `passgen_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `passgen_<version>_linux_arm64.tar.gz` |
| Windows x86-64 | `passgen_<version>_windows_amd64.zip` |
| Windows ARM64 | `passgen_<version>_windows_arm64.zip` |
| FreeBSD x86-64 | `passgen_<version>_freebsd_amd64.tar.gz` |

macOS / Linux:

```sh
tar -xzf passgen_*_darwin_arm64.tar.gz
sudo install -m 755 passgen_*/passgen /usr/local/bin/passgen
```

#### Verify the download

**Checksum.** Confirms the file wasn't corrupted or swapped:

```sh
shasum -a 256 -c checksums.txt --ignore-missing
```

**Build provenance (stronger).** Every release archive has a signed [GitHub attestation](https://docs.github.com/actions/security-for-github-actions/using-artifact-attestations) proving it was built by this repo's release workflow, from a specific commit, on GitHub's servers. It wasn't built on someone's laptop or altered afterwards. With the [GitHub CLI](https://cli.github.com):

```sh
gh attestation verify passgen_v0.2.2_darwin_arm64.tar.gz -R bankierubybank/passgen
```

A passing check prints `✓ Verification succeeded!` along with the workflow and commit that built the file. Releases from v0.2.2 onward have attestations.

From v0.3.1 on, the signed attestation is also a release file, `passgen_<version>.intoto.jsonl`, so you can verify without asking GitHub for it:

```sh
gh attestation verify passgen_v0.3.1_darwin_arm64.tar.gz -R bankierubybank/passgen \
  --bundle passgen_v0.3.1.intoto.jsonl
```

**SBOM.** From v0.3.0 on, each release also has `passgen_<version>_sbom.cdx.json`, a [CycloneDX](https://cyclonedx.org) list of exactly what is compiled in: passgen itself and the Go standard library version. It is covered by `checksums.txt` and the attestation like the archives.

**macOS "unidentified developer" warning:** the binaries are not notarized by Apple. To allow passgen to run:

```sh
xattr -d com.apple.quarantine /usr/local/bin/passgen
```

**Windows SmartScreen:** click *More info → Run anyway*.

### With Go

```sh
go install github.com/bankierubybank/passgen@latest
```

### From source

```sh
git clone https://github.com/bankierubybank/passgen.git
cd passgen
make build        # -> ./passgen
```

## Usage

```
passgen [options]
passgen -i                  interactive mode
passgen check               rate a password (reads stdin)
passgen completion SHELL    shell completion: bash, zsh, fish
passgen man                 man page
```

| Option | Description |
|---|---|
| `-l, --length N` | password length, 1-4096 (default 16) |
| `-n, --count N` | number of passwords, 1-10000 (default 1) |
| `-c, --complexity LEVEL` | `low` = a-z 0-9, `medium` = + A-Z, `high` = + symbols (default `high`) |
| `-s, --symbol-mode MODE` | `safe`, `standard` or `complex` (default `safe`), see below |
| `--symbol-set CHARS` | your own symbols (overrides `--symbol-mode`) |
| `--lower` / `--no-lower` | include/exclude a-z (overrides the preset) |
| `--upper` / `--no-upper` | include/exclude A-Z |
| `--digits` / `--no-digits` | include/exclude 0-9 |
| `--symbols` / `--no-symbols` | include/exclude symbols |
| `--no-ambiguous` | exclude look-alikes ``Il1O0o\|`'"`` |
| `--exclude CHARS` | extra characters to exclude |
| `--no-require-each` | don't force at least one char from each enabled set |
| `--copy` | copy the first password to the clipboard |
| `-q, --quiet` | print passwords only, no strength line |
| `-i, --interactive` | prompt for options, then regenerate/copy in a loop |
| `--list-symbols` | show the symbol sets |
| `-v, --version` | show version |
| `--check-update` | check GitHub for a newer release (see below) |

The strength line goes to stderr, so `passgen | pbcopy` and `pw=$(passgen)` capture only the password.

### Symbol modes

| Mode | Characters | Use when |
|---|---|---|
| `safe` (default) | `-_.+=@#%*?^` | Accepted by nearly every password policy. Pastes cleanly into shells, JSON, XML/HTML, URLs, CSV, SQL and `.env` files |
| `standard` | `!@#$%^&*()-_=+[]{};:,.<>?/~` | Wider set. Avoids quotes, backslash, backtick, pipe and space |
| `complex` | ``!"#$%&'()*+,-./:;<=>?@[\]^_`{\|}~`` | All printable ASCII symbols, for the most entropy per character. Some systems reject quotes, backtick, backslash, `\|`, `<`, `>`, `&`, `;` |

### Examples

```sh
passgen -l 24 -n 5                   # five 24-char passwords
passgen -c medium --no-ambiguous     # letters + digits, easy to read aloud
passgen -s complex -l 32 --copy      # max entropy, straight to clipboard
passgen -c low -l 6 --no-require-each --no-lower   # 6-digit PIN
passgen -i                           # interactive
```

### Interactive mode

```
$ passgen -i
passgen interactive. Enter keeps [default]. Ctrl-C to quit.

Length [16]: 20
How many [1]: 3
Complexity (low/medium/high) [high]:
Symbol mode (safe/standard/complex) [safe]:
Exclude look-alike characters (Il1O0o etc)? [y/N]: y

  1) Hx_9fRk=W2tc@EqZ+bMa
  2) ...
  3) ...

  [66 chars pool, ~121 bits entropy: very strong]

[Enter] = regenerate, c / 1-3 = copy, s = settings, q = quit:
```

Linux clipboard support needs `wl-copy` (Wayland), `xclip` or `xsel`.

### Checking a password

`passgen check` estimates how strong a password you already have is. It runs offline, like everything else, and sends nothing anywhere.

```
$ passgen check
Password to check (not shown):
Length:     13
Characters: lowercase, uppercase, digits, symbols (pool 95)
Estimate:   ~23 bits: weak
Warnings:
  - looks like a year
  - contains a common password or word
```

- Type the password at the prompt (hidden on macOS, Linux and BSD; visible on Windows) or pipe it in: `pbpaste | passgen check`.
- It never takes the password as an argument, since that would save it in your shell history.
- The estimate starts from brute force, then charges much less for what cracking tools try first: common passwords and words (also with l33t swaps like `p@ssw0rd`), years, repeated characters, sequences like `abcd` or `4321`, keyboard runs like `qwerty`, and repeats of an earlier part (`Password!Password!`). Making a pattern longer barely helps: `aaaa…` 100 times is still weak.
- The report never shows any part of the password, and Ctrl-C at the hidden prompt puts your terminal back to normal.
- Input is limited to 4096 characters.
- It is a **best case**. Real cracking tools use far bigger word lists, so a password made of ordinary words (`correct horse battery staple`) scores higher here than it deserves. A random password from passgen has none of these weaknesses.
- `--min-bits N` exits with status 3 when the estimate is below N, for scripts. `-q` prints the bits only.

### Shell completion

```sh
# bash: add to ~/.bashrc
source <(passgen completion bash)

# zsh: put it on your fpath (run once)
passgen completion zsh > "${fpath[1]}/_passgen"

# fish
passgen completion fish > ~/.config/fish/completions/passgen.fish
```

### Man page

The man page is built into the binary, and `passgen.1` is also in each release archive:

```sh
passgen man | man -l -          # Linux: read it now
sudo mkdir -p /usr/local/share/man/man1
passgen man | sudo tee /usr/local/share/man/man1/passgen.1 >/dev/null   # then: man passgen
```

### Checking for updates

```
$ passgen --check-update
Checking github.com/bankierubybank/passgen for the latest release...
New version available: v0.2.0 (you have v0.1.0)

Release notes: https://github.com/bankierubybank/passgen/releases/tag/v0.2.0
Download for this system: https://github.com/bankierubybank/passgen/releases/download/v0.2.0/passgen_v0.2.0_darwin_arm64.tar.gz
...
```

- passgen connects to the network only when you run `--check-update`. Generating passwords, including interactive mode, never does, and a test enforces this.
- The check is one `GET` to GitHub's public releases API. Nothing about you or your passwords is sent; GitHub sees only your IP address and the `passgen/<version>` user agent.
- It never downloads or installs anything. It tells you whether a newer release exists and where to get it; you download it and check it against `checksums.txt` yourself.

## Development

```sh
make test         # go vet + go test -race
make build        # binary for this machine
make dist         # archives for every platform + checksums.txt in dist/
make release-dist # the same plus the SBOM, as CI does for a release

# fuzz one target for a while (see fuzz_test.go for the list)
go test -run '^$' -fuzz '^FuzzCheckStrength$' -fuzztime 1m .
```

### Supply-chain safeguards

- **No dependencies:** Go standard library only.
- **Pinned actions:** GitHub Actions are pinned to full commit SHAs, so a moved tag can't change what runs. Dependabot proposes updates weekly.
- **Vulnerability scan:** [`govulncheck`](https://go.dev/doc/security/vuln/) runs on every push, weekly, and before each release. It fails if passgen calls code with a known vulnerability.
- **Static analysis:** [golangci-lint](https://golangci-lint.run) (staticcheck, errcheck, gosec and more) and [CodeQL](https://codeql.github.com) check the code; [zizmor](https://docs.zizmor.sh) checks the GitHub Actions workflows for injection, over-broad permissions and credential leaks.
- **Randomness tests:** chi-square tests check that every character is equally likely overall and at every position, and that the test itself catches planted and modulo bias.
- **Fuzzing:** Go fuzz tests feed random input to the generator, the command-line parser, the strength checker and the update check's version parser. CI fuzzes each one on every push and pull request.
- **OpenSSF Scorecard:** grades the repo's supply-chain practices weekly (badge at the top; details under *Security → Code scanning*).
- **Protected repo:** rulesets block force-push and deletion on `main`, require CI and signed commits, and let only the maintainer create or change `v*` release tags.
- **Signed commits and tags:** every commit and release tag is GPG-signed; GitHub shows them as *Verified*.
- **Release builds:** made with the latest stable Go, in CI only, only from commits already on `main`, with a signed build provenance attestation and an SBOM (see *Verify the download*).
- **Update check:** release tags must look like `v1.2.3` (letters, digits, dots and dashes only) before anything is printed, and links are built by passgen, not taken from GitHub's response.

### Releasing

Push a signed version tag. GitHub Actions runs the tests, builds every platform, and publishes a GitHub Release with the archives, the SBOM and `checksums.txt`:

```sh
git tag -s v1.0.0 -m "passgen v1.0.0"
git push origin v1.0.0
```

## Security

Found a security problem? Please report it privately, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
