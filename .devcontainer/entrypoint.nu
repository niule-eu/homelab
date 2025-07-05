#!/usr/bin/env nu 

let write_files = [
    {
        content: '{"Connection": {"Default": "host","Connections": {"host": {"URI": "unix://run/user/2000/podman/podman.sock"}}},"Farm": {}}
',
        dest: $"($env.HOME)/.config/containers/podman-connections.json"
    },
    {
        content: 'alias podman='podman-remote'
',
        dest: $"($env.HOME)/.bashrc.d/podman-alias"
    },
    {
        content: 'export PATH=$PATH:$GOPATH/bin
',
        dest: $"($env.HOME)/.bashrc.d/gopath-bin"
    }
]


$write_files | each {|el| 
    mkdir ($el.dest | path dirname)
    touch $el.dest
    $el.content | save --force $el.dest
} | ignore
