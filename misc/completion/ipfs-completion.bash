# License: MIT

_ipfs_comp ()
{
	local reply
	case "$cur" in
		-*)	reply="$(__ipfs_flag_list $*)
				--help" ;;
		*)	reply="$(__ipfs_sub_list $*)" ;;
	esac

	# no more sub-commands
	if [[ -z "$reply" ]]; then
		local last=${cseq[-1]}
		case "$last" in
			id|ping)
				reply="$(__ipfs_peers_list)"
				compopt -o nospace ;;
			cat|ls)
			# TODO: flesh this case out to handle completing within
			# a directory node.
				reply="$(__ipfs_pin_list)"
				compopt -o nospace ;;
			add|replace)
				compopt -o default ;;
		esac
	fi

	COMPREPLY=( $(compgen -W "$reply" -- "$cur") )
	return 0
}

__ipfs_pin_list() {
	ipfs refs local 2>/dev/null
}

__ipfs_peers_list ()
{
	ipfs swarm peers 2>/dev/null |sed -e 's/.*\/ipfs\///g'
}

# TODO: perhaps change these to use $(ipfs commands)
__ipfs_flag_list ()
{
	ipfs $* --help |egrep -o '\--[a-zA-Z0-9]+' |sort |uniq
}

__ipfs_sub_list ()
{
	local reg_1="^[[:space:]]+ipfs[[:space:]]$*[[:space:]]?[a-z]+-?[a-z]+"
	local reg_2="s/ipfs $*//g"
	ipfs $* --help |egrep -o "$reg_1" |sed -e "$reg_2" -e 's/ \+//g' \
		|sort |uniq
}

_ipfs ()
{
	COMPREPLY=()
	local w cur="${COMP_WORDS[COMP_CWORD]}"

	# everything after -- is a file
	for w in ${COMP_WORDS[@]:1:COMP_CWORD - 1}; do
		[[ "$w" = "--" ]] && compopt -o default && return 0
	done

	# save a copy of the command sequence
	local cseq=() d=0
	for w in ${COMP_WORDS[@]:1:COMP_CWORD - 1}; do
		case "$w" in
			-*)	continue ;;
			*)	cseq[d++]="$w"
		esac
	done

	# complete first subcommand
	local sub="${cseq[0]}"
	if [[ -z "$sub" ]]; then
		_ipfs_comp
	fi

	# command specific completion
	_ipfs_comp "${cseq[@]}"
}
complete -F _ipfs ipfs
