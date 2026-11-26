# passgen

Generate secure random passwords locally. A single binary with no dependencies that never touches the network.

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

Check the download against `checksums.txt`:

```sh
shasum -a 256 -c checksums.txt --ignore-missing
```

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

## Development

```sh
make test         # go vet + go test -race
make build        # binary for this machine
make dist         # archives for every platform + checksums.txt in dist/
```

### Releasing

Push a version tag. GitHub Actions runs the tests, builds every platform, and publishes a GitHub Release with the archives and `checksums.txt`:

```sh
git tag v1.0.0
git push origin v1.0.0
```

## License

[MIT](LICENSE)
