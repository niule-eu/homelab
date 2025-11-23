## m720q

### compile ingition and move to file server

``` bash
hlcli render-pkl -m infra/m720q/m720q.pkl | butane | ssh ignition@192.168.3.1 'cat > m720q.ign'
```

### Install to disk

``` bash
# get sha with
SHA=`hlcli render-pkl -m infra/m720q/m720q.pkl | butane | sha256sum | sd '(.*)\s+-\s' 'sha256-$1'`
```

``` bash
# execute on fedora coreos live instance
sudo coreos-installer install /dev/sda --ignition-url http://172.16.13.1:9714/m720q.ign --ignition-hash $SHA
```

## Radxa X4

### SSH Host keys

``` bash
hlcli keygen -o infra/radxax4/etc/ssh/ssh_host_ed25519_key -comment radxax4 ed25519
hlcli keygen -o infra/radxax4/etc/ssh/ssh_host_ecdsa_key -comment radxax4 ecdsa -b 384
hlcli keygen -o infra/radxax4/etc/ssh/ssh_host_rsa_key -comment radxax4 rsa -b 4096
```

### Create macvlan network

``` bash
podman network create -d macvlan --interface-name eno1 --ipam-driver host-local --subnet 172.16.13.0/24 --gateway 172.16.13.1 -o mode=bridge  macvlan-eno1
```

### DNS deployment

https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/

#### OVH API Key

``` bash
curl -XPOST -H"X-Ovh-Application: `sops decrypt --extract '["OVH_API_DNSCONTROL_KEY"]' secrets.yaml`" -H "Content-type: application/json" https://eu.api.ovh.com/1.0/auth/credential -d'{
  "accessRules": [
    {
      "method": "DELETE",
      "path": "/domain/zone/*"
    },
    {
      "method": "GET",
      "path": "/domain/zone/*"
    },
    {
      "method": "POST",
      "path": "/domain/zone/*"
    },
    {
      "method": "PUT",
      "path": "/domain/zone/*"
    },
    {
      "method": "GET",
      "path": "/domain/*"
    },
    {
      "method": "PUT",
      "path": "/domain/*"
    },
    {
      "method": "POST",
      "path": "/domain/*/nameServers/update"
    }
  ]
}'
```

#### dnscontrol

``` bash
hlcli render-pkl -m infra/dns/creds.pkl | dnscontrol push --config infra/dns/dnsconfig.js --creds /dev/stdin
```

### Generate a cert with lego

> Note: The lego state is stored in a podman named volume named "dot-lego". The contents of that volume are exported to `infra/certs/dot.lego.tar`.

``` bash
hlcli render-pkl -m infra/certs/dns.infra.ams23.niule.xyz.pkl | podman kube play --replace -
```

### DNS over TLS on Windows 11

How to manually set DNS in Windows 11:
* https://www.elevenforum.com/t/enable-dns-over-tls-dot-in-windows-11.9012/

How to configure DoT with netsh
``` cmd
netsh dns add global dot=yes
netsh dns add encryption server=172.16.13.2 dothost="dns.infra.ams23.niule.xyz" autoupgrade=yes
```
