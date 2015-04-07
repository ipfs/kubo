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
    _ipfs_comp "-recursive -quiet -progress -wrap-with-directory \
            -trickle --help"
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
    _ipfs_comp "-default --help"
}

_ipfs_bootstrap_list()
{
    _ipfs_help_only
}

_ipfs_bootstrap_rm()
{
    _ipfs_comp "-all --help"
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
    _ipfs_comp "edit replace show -bool --help"
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
    _ipfs_comp "-init -routing= -mount -writable -mount-ipfs= \
        -mount-ipns= --help"
}

_ipfs_dht()
{
    _ipfs_comp "findpeer findprovs query --help"
}

_ipfs_dht_findpeer()
{
    _ipfs_help_only
}

_ipfs_dht_findprovs()
{
    _ipfs_comp "-verbose --help"
}

_ipfs_dht_query()
{
    _ipfs_comp "-verbose --help"
}

_ipfs_diag()
{
    _ipfs_comp "net --help"
}

_ipfs_diag_net()
{
    # TODO: auto-complete -vis=*
    _ipfs_comp "-timeout= -vis= --help"
}

_ipfs_get()
{
    _ipfs_comp "-output= -archive -compress -compression-level= --help"
}

_ipfs_id()
{
    _ipfs_comp "-format= --help"
}

_ipfs_init()
{
    _ipfs_comp "-bits= -passphrase= -force --help"
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
    _ipfs_comp "tail level --help"
}

_ipfs_ls()
{
    _ipfs_help_only
}

_ipfs_mount()
{
    _ipfs_comp "-f= -n= --help"
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
    _ipfs_help_only
}

_ipfs_object()
{
    _ipfs_comp "data get links put stat --help"
}

_ipfs_object_data()
{
    _ipfs_help_only
}

_ipfs_object_get()
{
    # TODO: auto-complete encoding
    _ipfs_comp "--encoding="
}

_ipfs_object_links()
{
    _ipfs_help_only
}

_ipfs_object_put()
{
    _ipfs_help_only
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
    _ipfs_comp "-recursive  --help"
}

_ipfs_pin_ls()
{
    # TODO: auto-complete -type=*
    _ipfs_comp "-type= --help"
}

_ipfs_pin_rm()
{
    _ipfs_comp "-recursive  --help"
}

_ipfs_ping()
{
    _ipfs_comp "-count=  --help"
}

_ipfs_refs()
{
    _ipfs_comp "local -format= -edges -unique -recursive --help"
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
    _ipfs_comp "-quiet --help"
}

_ipfs_swarm()
{
    _ipfs_comp "addrs connect disconnect peers --help"
}

_ipfs_swarm_addrs()
{
    _ipfs_help_only
}

_ipfs_swarm_connect()
{
    _ipfs_help_only
}

_ipfs_swarm_disconnect()
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
    _ipfs_comp "check log --help"
}

_ipfs_update_check()
{
    _ipfs_help_only
}

_ipfs_update_log()
{
    _ipfs_help_only
}

_ipfs_version()
{
    _ipfs_comp "-number --help"
}

_ipfs()
{
    COMPREPLY=()
    local word="${COMP_WORDS[COMP_CWORD]}"
    
    case "${COMP_CWORD}" in
        1)  
            local opts="add block bootstrap cat commands config daemon dht diag get id \
                        init log ls mount name object pin ping refs repo swarm tour \
                        update version"
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
