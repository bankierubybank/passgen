package main

// Shell completion scripts, built from the real flag set so they always
// match what passgen accepts.
//
//   bash:  source <(passgen completion bash)
//   zsh:   passgen completion zsh > "${fpath[1]}/_passgen"
//   fish:  passgen completion fish > ~/.config/fish/completions/passgen.fish

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

var completionShells = []string{"bash", "zsh", "fish"}

// Values offered after flags that take one.
var flagValues = map[string][]string{
	"complexity":  complexityLevels,
	"symbol-mode": symbolModes,
}

// flagSpecs lists every main-command flag, sorted by long name.
func flagSpecs() []flagSpec {
	_, _, specs := newFlagSet(io.Discard)
	sort.Slice(specs, func(i, j int) bool { return specs[i].long < specs[j].long })
	return specs
}

func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "usage: passgen completion %s\n", strings.Join(completionShells, "|"))
		return 2
	}
	switch args[0] {
	case "bash":
		writeBash(stdout)
	case "zsh":
		writeZsh(stdout)
	case "fish":
		writeFish(stdout)
	default:
		fmt.Fprintf(stderr, "passgen completion: unknown shell %q (want %s)\n", args[0], strings.Join(completionShells, ", "))
		return 2
	}
	return 0
}

func subcommandNames() []string {
	var names []string
	for _, c := range subcommands {
		names = append(names, c[0])
	}
	return names
}

func writeBash(w io.Writer) {
	var words, valueFlags []string
	for _, s := range flagSpecs() {
		words = append(words, "--"+s.long)
		if s.short != "" {
			words = append(words, "-"+s.short)
		}
		if s.value {
			valueFlags = append(valueFlags, "--"+s.long)
			if s.short != "" {
				valueFlags = append(valueFlags, "-"+s.short)
			}
		}
	}
	fmt.Fprintf(w, `# bash completion for passgen. Load with: source <(passgen completion bash)
_passgen() {
    local cur prev
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    if [[ ${COMP_CWORD} -ge 2 ]]; then
        case "${COMP_WORDS[1]}" in
            check)
                COMPREPLY=($(compgen -W "--min-bits --quiet -q --help" -- "$cur"))
                return ;;
            completion)
                [[ ${COMP_CWORD} -eq 2 ]] && COMPREPLY=($(compgen -W "%s" -- "$cur"))
                return ;;
            man)
                return ;;
        esac
    fi

    case "$prev" in
        -c|--complexity)
            COMPREPLY=($(compgen -W "%s" -- "$cur"))
            return ;;
        -s|--symbol-mode)
            COMPREPLY=($(compgen -W "%s" -- "$cur"))
            return ;;
        %s)
            return ;;
    esac

    if [[ ${COMP_CWORD} -eq 1 && "$cur" != -* ]]; then
        COMPREPLY=($(compgen -W "%s" -- "$cur"))
        return
    fi
    COMPREPLY=($(compgen -W "%s --help" -- "$cur"))
}
complete -F _passgen passgen
`,
		strings.Join(completionShells, " "),
		strings.Join(complexityLevels, " "),
		strings.Join(symbolModes, " "),
		strings.Join(valueFlags, "|"),
		strings.Join(subcommandNames(), " "),
		strings.Join(words, " "))
}

// zshQuote makes text safe inside a single-quoted zsh _arguments spec.
func zshQuote(s string) string {
	r := strings.NewReplacer("'", `'\''`, "[", `\[`, "]", `\]`)
	return r.Replace(s)
}

func writeZsh(w io.Writer) {
	var specs []string
	for _, s := range flagSpecs() {
		arg := ""
		if s.value {
			if vals, ok := flagValues[s.long]; ok {
				arg = ":" + s.long + ":(" + strings.Join(vals, " ") + ")"
			} else {
				arg = ":" + s.long + ": "
			}
		}
		desc := "[" + zshQuote(s.usage) + "]"
		if s.short != "" {
			specs = append(specs, fmt.Sprintf("'(-%s --%s)'{-%s,--%s}'%s%s'", s.short, s.long, s.short, s.long, desc, arg))
		} else {
			specs = append(specs, fmt.Sprintf("'--%s%s%s'", s.long, desc, arg))
		}
	}
	var cmds []string
	for _, c := range subcommands {
		cmds = append(cmds, fmt.Sprintf("'%s:%s'", c[0], zshQuote(c[1])))
	}
	fmt.Fprintf(w, `#compdef passgen
# zsh completion for passgen. Install with:
#   passgen completion zsh > "${fpath[1]}/_passgen"

_passgen() {
    if (( CURRENT > 2 )); then
        case "${words[2]}" in
            check)
                _arguments '--min-bits[exit 3 if the estimate is below N bits]:bits: ' \
                    '(-q --quiet)'{-q,--quiet}'[print the estimated bits only]'
                return ;;
            completion)
                _values 'shell' %s
                return ;;
            man)
                return ;;
        esac
    fi
    if (( CURRENT == 2 )) && [[ "${words[CURRENT]}" != -* ]]; then
        local -a cmds
        cmds=(%s)
        _describe 'command' cmds
        return
    fi
    _arguments -s \
        %s
}

if [[ "$funcstack[1]" == "_passgen" ]]; then
    _passgen "$@"
else
    compdef _passgen passgen
fi
`,
		strings.Join(completionShells, " "),
		strings.Join(cmds, " "),
		strings.Join(specs, " \\\n        "))
}

// fishQuote makes text safe inside a single-quoted fish string.
func fishQuote(s string) string {
	return strings.NewReplacer(`\`, `\\`, "'", `\'`).Replace(s)
}

func writeFish(w io.Writer) {
	fmt.Fprintln(w, "# fish completion for passgen. Install with:")
	fmt.Fprintln(w, "#   passgen completion fish > ~/.config/fish/completions/passgen.fish")
	fmt.Fprintln(w, "complete -c passgen -f")
	noSub := "not __fish_seen_subcommand_from " + strings.Join(subcommandNames(), " ")
	for _, c := range subcommands {
		fmt.Fprintf(w, "complete -c passgen -n '%s' -a %s -d '%s'\n", noSub, c[0], fishQuote(c[1]))
	}
	for _, s := range flagSpecs() {
		line := fmt.Sprintf("complete -c passgen -n '%s' -l %s", noSub, s.long)
		if s.short != "" {
			line += " -s " + s.short
		}
		if s.value {
			line += " -r"
			if vals, ok := flagValues[s.long]; ok {
				line += " -a '" + strings.Join(vals, " ") + "'"
			}
		}
		line += " -d '" + fishQuote(s.usage) + "'"
		fmt.Fprintln(w, line)
	}
	fmt.Fprintln(w, "complete -c passgen -n '__fish_seen_subcommand_from check' -l min-bits -r -d 'exit 3 if the estimate is below N bits'")
	fmt.Fprintln(w, "complete -c passgen -n '__fish_seen_subcommand_from check' -l quiet -s q -d 'print the estimated bits only'")
	fmt.Fprintf(w, "complete -c passgen -n '__fish_seen_subcommand_from completion' -a '%s'\n", strings.Join(completionShells, " "))
}
