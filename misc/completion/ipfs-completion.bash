_ipfs_comp()
{
    COMPREPLY=( $(compgen -W "$1" -- ${word}) )
}

_ipfs_help_only()
{
    _ipfs_comp "--help"
}

_ipfs_add()
{
    if [ "${prev}" == "--chunker" ] ; then
        _ipfs_comp "placeholder1 placeholder2 placeholder3" # TODO: Give real options
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive --quiet --silent --progress --trickle --only-hash --wrap-with-directory --hidden --chunker --pin"
    else
        _ipfs_filesystem_complete
    fi
}

_ipfs_bitswap()
{
    ipfs_comp "stat wantlist --help"
}

_ipfs_bitswap_stat()
{
    _ipfs_help_only
}

_ipfs_bitswap_wantlist()
{
    ipfs_comp "--peer= --help"
}

_ipfs_bitswap_unwant()
{
    _ipfs_help_only
}

_ipfs_block()
{
    _ipfs_comp "get put stat --help"
}

_ipfs_block_get()
{
    _ipfs_hash_complete
}

_ipfs_block_put()
{
    _ipfs_filesystem_complete
}

_ipfs_block_stat()
{
    _ipfs_hash_complete
}

_ipfs_bootstrap()
{
    _ipfs_comp "add list rm --help"
}

_ipfs_bootstrap_add()
{
    _ipfs_comp "--default --help"
}

_ipfs_bootstrap_list()
{
    _ipfs_help_only
}

_ipfs_bootstrap_rm()
{
    _ipfs_comp "--all --help"
}

_ipfs_cat()
{
    if [[ ${prev} == */* ]] ; then
        COMPREPLY=() # Only one argument allowed
    elif [[ ${word} == */* ]] ; then
        _ipfs_hash_complete
    else
        _ipfs_pinned_complete
    fi
}

_ipfs_commands()
{
    _ipfs_comp "--flags --help"
}

_ipfs_config()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--bool --json"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    elif [[ ${prev} == "show" ]] || [[ ${prev} == "edit" ]] ; then
        COMPREPLY=()
    elif [[ ${prev} == "replace" ]] ; then
        _ipfs_filesystem_complete
    else
        _ipfs_comp "show edit replace
        Addresses.API" # TODO: Which configruation values is valid, add them?
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    fi
}

_ipfs_config_edit()
{
    _ipfs_help_only
}

_ipfs_config_replace()
{
    # TODO: auto-complete with a filename
    _ipfs_help_only
}

_ipfs_config_show()
{
    _ipfs_help_only
}

_ipfs_daemon()
{
    if [[ ${prev} == "--routing" ]] ; then
        _ipfs_comp "dht supernode"
    elif [[ ${prev} == "--mount-ipfs" ]] || [[ ${prev} == "--mount-ipns" ]] ; then
        _ipfs_filesystem_complete
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--init --routing= --mount --writable --mount-ipfs= \
            --mount-ipns= --unrestricted-api --disable-transport-encryption \
            --help"
    fi
}

_ipfs_dht()
{
    _ipfs_comp "findpeer findprovs get put query --help"
}

_ipfs_dht_findpeer()
{
    _ipfs_help_only
}

_ipfs_dht_findprovs()
{
    _ipfs_comp "--verbose --help"
}

_ipfs_dht_get()
{
    _ipfs_comp "--verbose --help"
}

_ipfs_dht_put()
{
    _ipfs_comp "--verbose --help"
}

_ipfs_dht_query()
{
    _ipfs_comp "--verbose --help"
}

_ipfs_diag()
{
    _ipfs_comp "sys cmds net --help"
}

_ipfs_diag_cmds()
{
    if [[ ${prev} == "clear" ]] ; then
        return 0
    elif [[ ${prev} == "set-time" ]] ; then
        return 0 # TODO: Set time, which format?
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--verbose --help"
    else
        _ipfs_comp "clear set-time"
    fi
}

_ipfs_diag_sys()
{
    _ipfs_help_only
}

_ipfs_diag_net()
{
    if [[ ${prev} == "--vis=" ]] ; then
        opts="d3 dot"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--timeout= --vis= --help"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    fi
}

_ipfs_dns()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive --help"
    fi
}

_ipfs_files()
{
    _ipfs_comp "mv rm flush read write cp ls mkdir stat"
}

_ipfs_files_mv()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive --flush"
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_rm()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive --flush"
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}
_ipfs_files_flush()
{
    if [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_read()
{
    if [[ ${prev} == "--count" ]] || [[ ${prev} == "--offset" ]] ; then
        COMPREPLY=() # Numbers, just keep it empty
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--offset --count"
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_write()
{
    if [[ ${prev} == "--count" ]] || [[ ${prev} == "--offset" ]] ; then # Dirty check
        COMPREPLY=() # Numbers, just keep it empty
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--offset --count --create --truncate"
    elif [[ ${prev} == /* ]] ; then
        _ipfs_filesystem_complete
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_cp()
{
    if [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_ls()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "-l"
    elif [[ ${prev} == /* ]] ; then
        COMPREPLY=() # Path exist
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_mkdir()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--parents"

    elif [[ ${prev} == /* ]] ; then
        COMPREPLY=() # Path exist
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_files_stat()
{
    if [[ ${prev} == /* ]] ; then
        COMPREPLY=() # Path exist
    elif [[ ${word} == /* ]] ; then
        _ipfs_files_complete
    else
        COMPREPLY=( / )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_file()
{
    if [[ ${prev} == "ls" ]] ; then
        _ipfs_hash_complete
    else
        _ipfs_comp "ls --help"
    fi
}

_ipfs_file_ls()
{
    _ipfs_help_only
}

_ipfs_get()
{
    if [ "${prev}" == "--output" ] ; then
        compopt -o default # Re-enable default file read
        COMPREPLY=()
    elif [ "${prev}" == "--compression-level" ] ; then
        opts="-1 1 2 3 4 5 6 7 8 9"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--output= --archive --compress --compression-level= --help"
    else
        _ipfs_hash_complete
    fi
}

_ipfs_id()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--format= --help"
    fi
}

_ipfs_init()
{
    _ipfs_comp "--bits --force --empty-repo"
}

_ipfs_log()
{
    _ipfs_comp "tail level --help"
}

_ipfs_log_level()
{
    # TODO: auto-complete subsystem and level
    _ipfs_comp "--help"
}

_ipfs_log_tail()
{
    _ipfs_help_only
}

_ipfs_ls()
{
    if [[ ${word} == -* ]] ; then
            opts="--headers --help"
            COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    else
        _ipfs_hash_complete
    fi
}

_ipfs_mount()
{
    if [[ ${prev} == "--ipfs-path" ]] || [[ ${prev} == "--ipns-path" ]] ; then
        _ipfs_filesystem_complete
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--ipfs-path= --ipns-path= --help"
    fi
}

_ipfs_name()
{
    _ipfs_comp "publish resolve --help"
}

_ipfs_name_publish()
{
    if [[ ${word} == "--lifetime" ]] || [[ ${word} == "--ttl" ]] ; then
        COMPREPLY=()
    elif [[ ${word} == -* ]] ; then
        opts="--resolve --lifetime --ttl --help"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    elif [[ ${word} == */ ]]; then
        _ipfs_hash_complete
    else
        _ipfs_pinned_complete
    fi
}

_ipfs_name_resolve()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive --help"
    fi
}

_ipfs_object()
{
    _ipfs_comp "data diff get links new patch put stat --help"
}

_ipfs_object_data()
{
    _ipfs_hash_complete
}

_ipfs_object_diff()
{
    _ipfs_hash_complete
}


_ipfs_object_get()
{
    if [ "${prev}" == "--encoding" ] ; then
        _ipfs_comp "protobuf json xml"
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--encoding --help"
    else
        _ipfs_hash_complete
    fi
}

_ipfs_object_links()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--headers --help"
    else
        _ipfs_hash_complete
    fi
}

_ipfs_object_new()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--help"
    else
        _ipfs_comp "unixfs-dir"
    fi
}

_ipfs_object_patch()
{
    if [[ -n "${COMP_WORDS[3]}" ]] ; then # Root merkledag object exist
        case "${COMP_WORDS[4]}" in
        append-data)
            _ipfs_help_only
            ;;
        add-link)
            if [[ ${word} == -* ]] && [[ ${prev} == "add-link" ]] ; then # Dirty check
                _ipfs_comp "--create"
            #else
                # TODO: Hash path autocomplete. This is tricky, can be hash or a name.
            fi
            ;;
        rm-link)
            _ipfs_hash_complete
            ;;
        set-data)
            _ipfs_filesystem_complete
            ;;
        *)
            _ipfs_comp "append-data add-link rm-link set-data"
            ;;
        esac
    else
        _ipfs_hash_complete
    fi
}

_ipfs_object_put()
{
    if [ "${prev}" == "--inputenc" ] ; then
        _ipfs_comp "protobuf json"
    elif [ "${prev}" == "--datafieldenc" ] ; then
        _ipfs_comp "text base64"
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--inputenc --datafieldenc --help"
    else
        _ipfs_hash_complete
    fi
}

_ipfs_object_stat()
{
    _ipfs_hash_complete
}

_ipfs_pin()
{
    _ipfs_comp "rm ls add --help"
}

_ipfs_pin_add()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive  --help"
    elif [[ ${word} == */ ]] && [[ ${word} != "/ipfs/" ]] ; then
        _ipfs_hash_complete
    fi
}

_ipfs_pin_ls()
{
    if [[ ${prev} == "--type" ]] || [[ ${prev} == "-t" ]] ; then
        _ipfs_comp "direct indirect recursive all"
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--count --quiet --type= --help"
    elif [[ ${word} == */ ]] && [[ ${word} != "/ipfs/" ]] ; then
        _ipfs_hash_complete
    fi
}

_ipfs_pin_rm()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive  --help"
    elif [[ ${word} == */ ]] && [[ ${word} != "/ipfs/" ]] ; then
        COMPREPLY=() # TODO: _ipfs_hash_complete() + List local pinned hashes as default?
    fi
}

_ipfs_ping()
{
    _ipfs_comp "--count=  --help"
}

_ipfs_refs()
{
    if [ "${prev}" == "--format" ] ; then
        _ipfs_comp "src dst linkname"
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "local --format= --edges --unique --recursive --help"
    #else
        # TODO: Use "ipfs ref" and combine it with autocomplete, see _ipfs_hash_complete
    fi
}

_ipfs_refs_local()
{
    _ipfs_help_only
}

_ipfs_repo()
{
    _ipfs_comp "fsck gc stat verify version --help"
}

_ipfs_repo_version()
{
    _ipfs_comp "--quiet --help"
}

_ipfs_repo_verify()
{
    _ipfs_help_only
}

_ipfs_repo_gc()
{
    _ipfs_comp "--quiet --help"
}

_ipfs_repo_stat()
{
    _ipfs_comp "--human --help"
}

_ipfs_repo_fsck()
{
    _ipfs_help_only
}

_ipfs_resolve()
{
    if [[ ${word} == /ipfs/* ]] ; then
        _ipfs_hash_complete
    elif [[ ${word} == /ipns/* ]] ; then
        COMPREPLY=() # Can't autocomplete ipns
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--recursive --help"
    else
        opts="/ipns/ /ipfs/"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
        [[ $COMPREPLY = */ ]] && compopt -o nospace
    fi
}

_ipfs_stats()
{
    _ipfs_comp "bw repo bitswap --help"
}

_ipfs_stats_bitswap()
{
    _ipfs_help_only
}

_ipfs_stats_bw()
{
    _ipfs_comp "--peer= --proto= --poll --interval= --help"
}

_ipfs_stats_repo()
{
    _ipfs_comp "--human --help"
}

_ipfs_swarm()
{
    _ipfs_comp "addrs connect disconnect filters peers --help"
}

_ipfs_swarm_addrs()
{
    _ipfs_comp "local --help"
}

_ipfs_swarm_addrs_local()
{
    _ipfs_comp "--id --help"
}

_ipfs_swarm_connect()
{
    _ipfs_multiaddr_complete
}

_ipfs_swarm_disconnect()
{
    local OLDIFS="$IFS" ; local IFS=$'\n' # Change divider for iterator one line below
    opts=$(for x in `ipfs swarm peers`; do echo ${x} ; done)
    IFS="$OLDIFS" # Reset divider to space, ' '
    COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    [[ $COMPREPLY = */ ]] && compopt -o nospace -o filenames
}

_ipfs_swarm_filters()
{
    if [[ ${prev} == "add" ]] || [[ ${prev} == "rm" ]]; then
        _ipfs_multiaddr_complete
    else
        _ipfs_comp "add rm --help"
    fi
}

_ipfs_swarm_filters_add()
{
    _ipfs_help_only
}

_ipfs_swarm_filters_rm()
{
    _ipfs_help_only
}

_ipfs_swarm_peers()
{
    _ipfs_help_only
}

_ipfs_tar()
{
    _ipfs_comp "add cat --help"
}

_ipfs_tar_add()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--help"
    else
        _ipfs_filesystem_complete
    fi
}

_ipfs_tar_cat()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--help"
    else
        _ipfs_filesystem_complete
    fi
}

_ipfs_tour()
{
    _ipfs_comp "list next restart --help"
}

_ipfs_tour_list()
{
    _ipfs_help_only
}

_ipfs_tour_next()
{
    _ipfs_help_only
}

_ipfs_tour_restart()
{
    _ipfs_help_only
}

_ipfs_update()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--version" # TODO: How does "--verbose" option work?
    else
        _ipfs_comp "versions version install stash revert fetch"
    fi
}

_ipfs_update_install()
{
    if   [[ ${prev} == v*.*.* ]] ; then
        COMPREPLY=()
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--version"
    else
        local OLDIFS="$IFS" ; local IFS=$'\n' # Change divider for iterator one line below
        opts=$(for x in `ipfs update versions`; do echo ${x} ; done)
        IFS="$OLDIFS" # Reset divider to space, ' '
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    fi
}

_ipfs_update_stash()
{
    if [[ ${word} == -* ]] ; then
        _ipfs_comp "--tag --help"
    fi
}
_ipfs_update_fetch()
{
    if [[ ${prev} == "--output" ]] ; then
        _ipfs_filesystem_complete
    elif [[ ${word} == -* ]] ; then
        _ipfs_comp "--output --help"
    fi
}

_ipfs_version()
{
    _ipfs_comp "--number --commit --repo"
}

_ipfs_hash_complete()
{
    local lastDir=${word%/*}/
    echo "LastDir: ${lastDir}" >> ~/Downloads/debug-ipfs.txt
    local OLDIFS="$IFS" ; local IFS=$'\n' # Change divider for iterator one line below
    opts=$(for x in `ipfs file ls ${lastDir}`; do echo ${lastDir}${x}/ ; done) # TODO: Implement "ipfs file ls -F" to get rid of frontslash after files. This take long time to run first time on a new shell.
    echo "Options: ${opts}" >> ~/Downloads/debug-ipfs.txt
    IFS="$OLDIFS" # Reset divider to space, ' '
    echo "Current: ${word}" >> ~/Downloads/debug-ipfs.txt
    COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    echo "Suggestion: ${COMPREPLY}" >> ~/Downloads/debug-ipfs.txt
    [[ $COMPREPLY = */ ]] && compopt -o nospace -o filenames # Removing whitespace after output & handle output as filenames. (Only printing the latest folder of files.)
    return 0
}

_ipfs_files_complete()
{
    local lastDir=${word%/*}/
    local OLDIFS="$IFS" ; local IFS=$'\n' # Change divider for iterator one line below
    opts=$(for x in `ipfs files ls ${lastDir}`; do echo ${lastDir}${x}/ ; done) # TODO: Implement "ipfs files ls -F" to get rid of frontslash after files. This does currently throw "Error: /cats/foo/ is not a directory"
    IFS="$OLDIFS" # Reset divider to space, ' '
    COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    [[ $COMPREPLY = */ ]] && compopt -o nospace -o filenames
    return 0
}

_ipfs_multiaddr_complete()
{
    local lastDir=${word%/*}/
    # Special case
    if [[ ${word} == */"ipcidr"* ]] ; then # TODO: Broken, fix it.
        opts="1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32" # TODO: IPv6?
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    # "Loop"
    elif [[ ${word} == /*/ ]] || [[ ${word} == /*/* ]] ; then
        if [[ ${word} == /*/*/*/*/*/ ]] ; then
            COMPREPLY=()
        elif [[ ${word} == /*/*/*/*/ ]] ; then
            word=${word##*/}
            opts="ipfs/ "
            COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
        elif [[ ${word} == /*/*/*/ ]] ; then
            word=${word##*/}
            opts="4001/ "
            COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
        elif [[ ${word} == /*/*/ ]] ; then
            word=${word##*/}
            opts="udp/ tcp/ ipcidr/"
            COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
        elif [[ ${word} == /*/ ]] ; then
            COMPREPLY=() # TODO: This need to return something to NOT break the function. Maybe a "/" in the end as well due to -o filename option.
        fi
        COMPREPLY=${lastDir}${COMPREPLY}
    else # start case
        opts="/ip4/ /ip6/"
        COMPREPLY=( $(compgen -W "${opts}" -- ${word}) )
    fi
    [[ $COMPREPLY = */ ]] && compopt -o nospace -o filenames
    return 0
}

_ipfs_pinned_complete()
{
    local OLDIFS="$IFS" ; local IFS=$'\n'
    local pinned=$(ipfs pin ls)
    COMPREPLY=( $(compgen -W "${pinned}" -- ${word}) )
    IFS="$OLDIFS"
    if [[ ${#COMPREPLY[*]} -eq 1 ]]; then # Only one completion, remove pretty output
        COMPREPLY=( ${COMPREPLY[0]/ *//} ) #Remove ' ' and everything after
        [[ $COMPREPLY = */ ]] && compopt -o nospace  # Removing whitespace after output
    fi
}
_ipfs_filesystem_complete()
{
    compopt -o default # Re-enable default file read
    COMPREPLY=()
}

_ipfs()
{
    COMPREPLY=()
    compopt +o default # Disable default to not deny completion, see: http://stackoverflow.com/a/19062943/1216348

    local word="${COMP_WORDS[COMP_CWORD]}"
    local prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "${COMP_CWORD}" in
        1)
            local opts="add bitswap block bootstrap cat commands config daemon dht \
                        diag dns file files get id init log ls mount name object pin ping \
                        refs repo stats swarm tar tour update version"
            COMPREPLY=( $(compgen -W "${opts}" -- ${word}) );;
        2)
            local command="${COMP_WORDS[1]}"
            eval "_ipfs_$command" 2> /dev/null ;;
        *)
            local command="${COMP_WORDS[1]}"
            local subcommand="${COMP_WORDS[2]}"
            eval "_ipfs_${command}_${subcommand}" 2> /dev/null && return
            eval "_ipfs_$command" 2> /dev/null ;;
    esac
}
complete -F _ipfs ipfs
