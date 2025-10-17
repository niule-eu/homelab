## m720q

### compile ingition and move to file server

``` bash
hlcli render-pkl -m infra/m720q/m720q.pkl | butane | ssh ignition@192.168.3.1 'cat > m720q.ign'
```

### Install to disk

```bash
sudo coreos-installer install /dev/sda --ignition-url http://172.16.13.1:9714/m720q.ign --ignition-hash sha256-ffd5bef238cc26dd43e15b647f1f7bd9c54265bfe2918b6631a34b4bc019ea5e
```

## Radxa X4

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

> Note: The lego state is stored in a podman named volume named "got-lego". The contents of that volume are exported to `infra/certs/dot.lego.tar`.

``` bash
hlcli render-pkl -m infra/certs/dns.infra.ams23.niule.xyz.pkl | podman kube play --replace -
```