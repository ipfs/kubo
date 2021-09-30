package commands

import (
	"io"
	"sort"
	"text/template"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

type completionCommand struct {
	Name         string
	Subcommands  []*completionCommand
	ShortFlags   []string
	ShortOptions []string
	LongFlags    []string
	LongOptions  []string
}

func commandToCompletions(name string, cmd *cmds.Command) *completionCommand {
	parsed := &completionCommand{
		Name: name,
	}
	for name, subCmd := range cmd.Subcommands {
		parsed.Subcommands = append(parsed.Subcommands, commandToCompletions(name, subCmd))
	}
	sort.Slice(parsed.Subcommands, func(i, j int) bool {
		return parsed.Subcommands[i].Name < parsed.Subcommands[j].Name
	})

	for _, opt := range cmd.Options {
		if opt.Type() == cmds.Bool {
			parsed.LongFlags = append(parsed.LongFlags, opt.Name())
			for _, name := range opt.Names() {
				if len(name) == 1 {
					parsed.ShortFlags = append(parsed.ShortFlags, name)
					break
				}
			}
		} else {
			parsed.LongOptions = append(parsed.LongOptions, opt.Name())
			for _, name := range opt.Names() {
				if len(name) == 1 {
					parsed.ShortOptions = append(parsed.ShortOptions, name)
					break
				}
			}
		}
	}
	sort.Slice(parsed.LongFlags, func(i, j int) bool {
		return parsed.LongFlags[i] < parsed.LongFlags[j]
	})
	sort.Slice(parsed.ShortFlags, func(i, j int) bool {
		return parsed.ShortFlags[i] < parsed.ShortFlags[j]
	})
	sort.Slice(parsed.LongOptions, func(i, j int) bool {
		return parsed.LongOptions[i] < parsed.LongOptions[j]
	})
	sort.Slice(parsed.ShortOptions, func(i, j int) bool {
		return parsed.ShortOptions[i] < parsed.ShortOptions[j]
	})
	return parsed
}

var bashCompletionTemplate *template.Template

func init() {
	commandTemplate := template.Must(template.New("command").Parse(`
while [[ ${index} -lt ${COMP_CWORD} ]]; do
    case "${COMP_WORDS[index]}" in
        -*)
	    let index++
            continue
	    ;;
    {{ range .Subcommands }}
	"{{ .Name }}")
	    let index++
	    {{ template "command" . }}
	    return 0
            ;;
    {{ end }}
    esac
    break
done

if [[ "${word}" == -* ]]; then
{{ if .ShortFlags -}}
    _ipfs_compgen -W $'{{ range .ShortFlags }}-{{.}} \n{{end}}' -- "${word}"
{{ end -}}
{{- if .ShortOptions -}}
    _ipfs_compgen -S = -W $'{{ range .ShortOptions }}-{{.}}\n{{end}}' -- "${word}"
{{ end -}}
{{- if .LongFlags -}}
    _ipfs_compgen -W $'{{ range .LongFlags }}--{{.}} \n{{end}}' -- "${word}"
{{ end -}}
{{- if .LongOptions -}}
    _ipfs_compgen -S = -W $'{{ range .LongOptions }}--{{.}}\n{{end}}' -- "${word}"
{{ end -}}
    return 0
fi

while [[ ${index} -lt ${COMP_CWORD} ]]; do
    if [[ "${COMP_WORDS[index]}" != -* ]]; then
        let argidx++
    fi
    let index++
done

{{- if .Subcommands }}
if [[ "${argidx}" -eq 0 ]]; then
    _ipfs_compgen -W $'{{ range .Subcommands }}{{.Name}} \n{{end}}' -- "${word}"
fi
{{ end -}}
`))

	bashCompletionTemplate = template.Must(commandTemplate.New("root").Parse(`#!/bin/bash

_ipfs_compgen() {
  local oldifs="$IFS"
  IFS=$'\n'
  while read -r line; do
    COMPREPLY+=("$line")
  done < <(compgen "$@")
  IFS="$oldifs"
}

_ipfs() {
  COMPREPLY=()
  local index=1
  local argidx=0
  local word="${COMP_WORDS[COMP_CWORD]}"
  {{ template "command" . }}
}
complete -o nosort -o nospace -o default -F _ipfs ipfs
`))
}

// writeBashCompletions generates a bash completion script for the given command tree.
func writeBashCompletions(cmd *cmds.Command, out io.Writer) error {
	cmds := commandToCompletions("ipfs", cmd)
	return bashCompletionTemplate.Execute(out, cmds)
}
