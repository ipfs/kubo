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
    _ipfs_comp "--recursive --quiet --progress --trickle --only-hash
            --wrap-with-directory --hidden --help"
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

_ipfs_block()
{
    _ipfs_comp "get put stat --help"
}

_ipfs_block_get()
{
    _ipfs_help_only
}

_ipfs_block_put()
{
    _ipfs_help_only
}

_ipfs_block_stat()
{
    _ipfs_help_only
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
    _ipfs_help_only
}

_ipfs_commands()
{
    _ipfs_help_only
}

_ipfs_config()
{
    # TODO: auto-complete existing config keys
    _ipfs_comp "edit replace show --bool --json --help"
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
    _ipfs_comp "--init --routing= --mount --writable --mount-ipfs= \
        --mount-ipns= --unrestricted-api --disable-transport-encryption \
        --help"
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
    _ipfs_comp "net --help"
}

_ipfs_diag_net()
{
    # TODO: auto-complete -vis=*
    _ipfs_comp "--timeout= --vis= --help"
}

_ipfs_dns()
{
    _ipfs_comp "--recursive --help"
}

_ipfs_file()
{
    _ipfs_comp "ls --help"
}

_ipfs_file_ls()
{
    _ipfs_help_only
}

_ipfs_get()
{
    _ipfs_comp "--output= --archive --compress --compression-level= --help"
}

_ipfs_id()
{
    _ipfs_comp "--format= --help"
}

_ipfs_init()
{
    _ipfs_comp "--bits= --force --help"
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
    _ipfs_comp "--headers --help"
}

_ipfs_mount()
{
    _ipfs_comp "--ipfs-path= --ipns-path= --help"
}

_ipfs_name()
{
    _ipfs_comp "publish resolve --help"
}

_ipfs_name_publish()
{
    _ipfs_help_only
}

_ipfs_name_resolve()
{
    _ipfs_comp "--recursive --help"
}

_ipfs_object()
{
    _ipfs_comp "data get links new patch put stat --help"
}

_ipfs_object_data()
{
    _ipfs_help_only
}

_ipfs_object_get()
{
    # TODO: auto-complete encoding
    _ipfs_comp "--encoding= --help"
}

_ipfs_object_links()
{
    _ipfs_help_only
}

_ipfs_object_new()
{
    _ipfs_help_only
}

_ipfs_object_patch()
{
    _ipfs_help_only
}

_ipfs_object_put()
{
    _ipfs_comp "--inputenc= --help"
}

_ipfs_object_stat()
{
    _ipfs_help_only
}

_ipfs_pin()
{
    _ipfs_comp "rm ls add --help"
}

_ipfs_pin_add()
{
    _ipfs_comp "--recursive  --help"
}

_ipfs_pin_ls()
{
    # TODO: auto-complete -type=*
    _ipfs_comp "--count --quiet --type= --help"
}

_ipfs_pin_rm()
{
    _ipfs_comp "--recursive  --help"
}

_ipfs_ping()
{
    _ipfs_comp "--count=  --help"
}

_ipfs_refs()
{
    _ipfs_comp "local --format= --edges --unique --recursive --help"
}

_ipfs_refs_local()
{
    _ipfs_help_only
}

_ipfs_repo()
{
    _ipfs_comp "gc --help"
}

_ipfs_repo_gc()
{
    _ipfs_comp "--quiet --help"
}

_ipfs_resolve()
{
    _ipfs_comp "--recursive --help"
}

_ipfs_stats()
{
    _ipfs_comp "bw --help"
}

_ipfs_stats_bw()
{
    _ipfs_comp "--peer= --proto= --poll --interval= --help"
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
    _ipfs_help_only
}

_ipfs_swarm_disconnect()
{
    _ipfs_help_only
}

_ipfs_swarm_filters()
{
    _ipfs_comp "add rm --help"
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
    _ipfs_help_only
}

_ipfs_version()
{
    _ipfs_comp "--number --help"
}

_ipfs()
{
    COMPREPLY=()
    local word="${COMP_WORDS[COMP_CWORD]}"

    case "${COMP_CWORD}" in
        1)
            local opts="add bitswap block bootstrap cat commands config daemon dht \
                        diag dns file get id init log ls mount name object pin ping \
                        refs repo stats swarm tour update version"
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
