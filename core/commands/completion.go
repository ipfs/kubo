package commands

import (
	"io"
	"sort"
	"text/template"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

type completionCommand struct {
	Name         string
	FullName     string
	Description  string
	Subcommands  []*completionCommand
	Flags        []*singleOption
	Options      []*singleOption
	ShortFlags   []string
	ShortOptions []string
	LongFlags    []string
	LongOptions  []string
	IsFinal      bool
}

type singleOption struct {
	LongNames   []string
	ShortNames  []string
	Description string
}

func commandToCompletions(name string, fullName string, cmd *cmds.Command) *completionCommand {
	parsed := &completionCommand{
		Name:        name,
		FullName:    fullName,
		Description: cmd.Helptext.Tagline,
		IsFinal:     len(cmd.Subcommands) == 0,
	}
	for name, subCmd := range cmd.Subcommands {
		parsed.Subcommands = append(parsed.Subcommands,
			commandToCompletions(name, fullName+" "+name, subCmd))
	}
	sort.Slice(parsed.Subcommands, func(i, j int) bool {
		return parsed.Subcommands[i].Name < parsed.Subcommands[j].Name
	})

	for _, opt := range cmd.Options {
		flag := &singleOption{Description: opt.Description()}
		flag.LongNames = append(flag.LongNames, opt.Name())
		if opt.Type() == cmds.Bool {
			parsed.LongFlags = append(parsed.LongFlags, opt.Name())
			for _, name := range opt.Names() {
				if len(name) == 1 {
					parsed.ShortFlags = append(parsed.ShortFlags, name)
					flag.ShortNames = append(flag.ShortNames, name)
					break
				}
			}
			parsed.Flags = append(parsed.Flags, flag)
		} else {
			parsed.LongOptions = append(parsed.LongOptions, opt.Name())
			for _, name := range opt.Names() {
				if len(name) == 1 {
					parsed.ShortOptions = append(parsed.ShortOptions, name)
					flag.ShortNames = append(flag.ShortNames, name)
					break
				}
			}
			parsed.Options = append(parsed.Options, flag)
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

var bashCompletionTemplate, fishCompletionTemplate, zshCompletionTemplate *template.Template

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

	zshCompletionTemplate = template.Must(commandTemplate.New("root").Parse(`#!bin/zsh
autoload bashcompinit
bashcompinit
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

	fishCommandTemplate := template.Must(template.New("command").Parse(`
{{- if .IsFinal -}}
complete -c ipfs -n '__fish_ipfs_seen_all_subcommands_from{{ .FullName }}' -F
{{ end -}}
{{- range .Flags -}}
    complete -c ipfs -n '__fish_ipfs_seen_all_subcommands_from{{ $.FullName }}' {{ range .ShortNames }}-s {{.}} {{end}}{{ range .LongNames }}-l {{.}} {{end}}-d "{{ .Description }}"
{{ end -}}
{{- range .Options -}}
    complete -c ipfs -n '__fish_ipfs_seen_all_subcommands_from{{ $.FullName }}' -r {{ range .ShortNames }}-s {{.}} {{end}}{{ range .LongNames }}-l {{.}} {{end}}-d "{{ .Description }}"
{{ end -}}

{{- range .Subcommands }}
#{{ .FullName }}
complete -c ipfs -n '__fish_ipfs_use_subcommand{{ .FullName }}' -a {{ .Name }} -d "{{ .Description }}"
{{ template "command" . }}
{{ end -}}
	`))
	fishCompletionTemplate = template.Must(fishCommandTemplate.New("root").Parse(`#!/usr/bin/env fish
function __fish_ipfs_seen_all_subcommands_from
     set -l cmd (commandline -poc)
     set -e cmd[1]
     for c in $argv
         if not contains -- $c $cmd
               return 1
        end
     end
     return 0
end

function __fish_ipfs_use_subcommand
	set -e argv[-1]
	set -l cmd (commandline -poc)
	set -e cmd[1]
	for i in $cmd
	    switch $i
		    case '-*'
			    continue
            case $argv[1]
                set argv $argv[2..]
                continue
            case '*'
                return 1
        end
	end
	test -z "$argv"
end

complete -c ipfs -l help -d "Show the full command help text."

complete -c ipfs --keep-order --no-files

{{ template "command" . }}
`))
}

// writeBashCompletions generates a bash completion script for the given command tree.
func writeBashCompletions(cmd *cmds.Command, out io.Writer) error {
	cmds := commandToCompletions("ipfs", "", cmd)
	return bashCompletionTemplate.Execute(out, cmds)
}

// writeFishCompletions generates a fish completion script for the given command tree.
func writeFishCompletions(cmd *cmds.Command, out io.Writer) error {
	cmds := commandToCompletions("ipfs", "", cmd)
	return fishCompletionTemplate.Execute(out, cmds)
}

func writeZshCompletions(cmd *cmds.Command, out io.Writer) error {
	cmds := commandToCompletions("ipfs", "", cmd)
	return zshCompletionTemplate.Execute(out, cmds)
}
