# passgen

Generate secure random passwords locally. A single binary with no dependencies that works fully offline. The only network call is an update check, and it runs only when you ask for it.

- Randomness comes from the OS's cryptographic random source (Go `crypto/rand`), with unbiased selection
- Complexity presets, plus per-character-set control
- Three symbol sets, from "accepted everywhere" to "every printable symbol"
- Can exclude look-alike characters (`Il1O0o`…) or any characters you list
- Shows estimated entropy and a strength label
- Clipboard copy on macOS, Windows and Linux
- Interactive mode

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
passgen -i              interactive mode
```

| Option | Description |
|---|---|
| `-l, --length N` | password length (default 16) |
| `-n, --count N` | number of passwords (default 1) |
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
```

### Supply-chain safeguards

- **No dependencies:** Go standard library only.
- **Pinned actions:** GitHub Actions are pinned to full commit SHAs, so a moved tag can't change what runs. Dependabot proposes updates weekly.
- **Vulnerability scan:** [`govulncheck`](https://go.dev/doc/security/vuln/) runs on every push, weekly, and before each release. It fails if passgen calls code with a known vulnerability.
- **Static analysis:** [golangci-lint](https://golangci-lint.run) (staticcheck, errcheck, gosec and more) and [CodeQL](https://codeql.github.com) check the code; [zizmor](https://docs.zizmor.sh) checks the GitHub Actions workflows for injection, over-broad permissions and credential leaks.
- **Randomness tests:** chi-square tests check that every character is equally likely overall and at every position, and that the test itself catches planted and modulo bias.
- **Protected repo:** rulesets block force-push and deletion on `main`, require CI on pull requests, and let only the maintainer create or change `v*` release tags.
- **Release builds:** made with the latest stable Go, in CI only, with a signed build provenance attestation (see *Verify the download*).

### Releasing

Push a version tag. GitHub Actions runs the tests, builds every platform, and publishes a GitHub Release with the archives and `checksums.txt`:

```sh
git tag v1.0.0
git push origin v1.0.0
```

## License

[MIT](LICENSE)
