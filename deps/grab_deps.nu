let keywords = {
    arch: [
        {id: 'aarch64', variants: ['aarch64']},
        {id: 'amd64', variants: ['x86_64', 'amd64', 'x64', '64bit']}
    ]
    os: [
        {id: 'linux', variants: ['linux']}
    ]
    abi: [
        {id: 'musl', variants: ['musl']},
        {id: 'gnu', variants: ['gnu']},
    ]
    checksums: [
        'SHA256SUMS$',
        '.*checksums(\.txt){0,1}$',
        '.*checksums.*sha256.*(\.txt){0,1}$',
        'shasums'
    ]
}

let artifact_excludes = [
    'json',
    'yaml',
    'yml',
    'txt'
]

let config = open deps.yaml

let gh_releases = $config.github_releases

let arch = $keywords.arch | filter {|el| $config.target_triplet.arch in $el.variants } | first | get id 
let os = $keywords.os | filter {|el| $config.target_triplet.os in $el.variants } | first | get id 
let abi = $keywords.abi | filter {|el| $config.target_triplet.abi in $el.variants } | first | get id 

def match_triplet [filename: string] {
    let kwsrx = {|kws: list<string>, capture_group: string| $"\(?P<($capture_group)>($kws | str join '|')\).*\(?<!($artifact_excludes | str join '|')\)$" }

    def inner [component: string] {
        let options = $keywords | get $component | get id
        $options 
            | each {|option|
                let kws = $keywords | get $component | where id == $option | get variants | first
                $filename 
                | parse --regex (do $kwsrx $kws $component)
                | match $in {
                    [] => ( null ),
                    [$v] => ( {$component: $option, value: ($v | get $component)} )
                }
            }
        }
    
    let arch = inner "arch" 
        | match $in {
            [] => false,
            _ => ($in | where arch == $arch | is-not-empty)
        }

    let os = inner "os" 
        | match $in {
            [] => false,
            _ => ($in | where os == $os | is-not-empty)
        }
    
    let abi = inner "abi" 
        | match $in {
            [] => true,
            _ => ($in | where abi == $abi | is-not-empty)
        }

    $arch and $os and $abi
}

def is_semver_prerelease [tag: string] {
    let svrx = '^(v{0,1})(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$'
    let parsed = $tag | parse --regex $svrx
    not($parsed.prerelease | command | is-empty)
}

def gh_releases_api [gh_release: record<repo: string, owner: string>] {
    $"https://api.github.com/repos/($gh_release.owner)/($gh_release.repo)/releases"
}

def golang_releases_api [path: string] {
    $"https://go.dev/dl/($path)?mode=json"
}

def golang_latest [] {
    let response = http get (golang_releases_api "" )
    let asset = $response.0.files 
        | filter {|el| match_triplet $el.filename }
        | match $in {
            [$item] => ($item),
            [] => (exit 1),
            _ => (exit 1)
        }
    {
        GOLANG_TAG: $asset.version, 
        GOLANG_HASH: $asset.sha256, 
        GOLANG_URL: (golang_releases_api $asset.filename)
    }
}

def github_auth_header [] {
    match ($env | get -i GITHUB_TOKEN) {
        null => {[]}
        $val => {["Authorization" $"Bearer ($val)"]}
    }
}

def github_get [url: string, extra_headers: list<string> = []] {
    http get --headers (github_auth_header | append $extra_headers) $url
}

def gh_latest [gh_release: record<repo: string, owner: string>] {
    let response = github_get (gh_releases_api $gh_release)
    let release = $response 
        | skip until {|el| not (is_semver_prerelease $el.tag_name) }
        | first
        | select tag_name assets
    let asset = $release.assets
        | filter {|el| match_triplet $el.name }
        | first
        | select url name
    let checksum_asset = match ($gh_release | get -i checksum) {
        null | true => {
            $keywords.checksums
            | each {|el| $release.assets | find --regex $el }
            | filter {|el| not ($el | is-empty) }
            | first | select url name | first
        }
        false => null
    }
    let asset_hash = match $checksum_asset {
        null => null
        _ => {
            (github_get $checksum_asset.url ["Accept" "application/octet-stream"]) 
            | decode utf-8
            | split row "\n"
            | filter {|el| not ($el | is-empty) }
            | each {|el| $el | split row -r '\s+' }
            | skip until {|el| $el.1 == $asset.name }
            | first | first
        }
    }
        
    let key = {|s: string| $"($gh_release.repo)_($s)" | str upcase }
    $"($gh_release.owner)/($gh_release.repo) # ($release.tag_name)" | print
    match $asset_hash {
        null => {{}}
        _ => {
            $"(do $key 'hash')": $asset_hash,
        }
    } 
    | $in | insert $"(do $key 'tag')" { $release.tag_name }
    | $in | insert $"(do $key 'url')" { $asset.url }
    
}

# let gh_api_remaining = http head --headers (github_auth_header) "https://api.github.com/rate_limit"

# if ($gh_api_remaining | get x-ratelimit-remaining) == 0 {
#     error make {msg: "GitHub ratelimit exceeded"}
# }

# let argfile = open "argfile.conf"
golang_latest | to toml | save --force "argfile.conf"
$gh_releases | each {|el| 
    gh_latest $el 
    | to toml | save --append "argfile.conf"
} | ignore 
open "argfile.conf" | str replace --all " " "" | save --force "argfile.conf"
