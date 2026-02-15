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

### k0s

#### Installing k0s

1. Create macvlan network with ipam set to dhcp (arguable, static IP with DNS entry prbly more reliable)
2. Create cluster with `podman kube play`
    ``` bash
    hlcli render-pkl -m k0s.pkl | podman -c radxax4_root kube play --network systemd-k0s-macvlan:mac=62:68:1d:91:bf:c4 --replace -
    ```

#### Installing flux 

1. Get GitHub PAT with all `repo` permissions
2. Use `flux bootstrap github` to install flux controllers to cluster
    ``` bash
    sops exec-env secrets.yaml 'echo $FLUX_GITHUB_PAT | flux bootstrap github --token-auth --private=false --owner=niule-eu --branch=main --personal --path=clusters/k0s-single --repository=k0s-github-arc --components-extra=source-watcher'
    ```
3. Generate age key for decryption in cluster
    ``` bash
    age-keygen | xargs -I {} sops set secrets.yaml '["K0S_CONTROLLER_AGE_KEY"]' '"{}"'
    ```
4. Apply sops-age secret to cluster
    ``` bash
    hlcli render-pkl -m infra/k0s-github-arc/sops-age.pkl | kubectl apply -f -
    ```
5. Patch `flux-system` kustomization to use sops decryption
    ``` yaml
    # clusters/k0s-single/flux-system/kustomization.yaml
    ---
    apiVersion: kustomize.config.k8s.io/v1beta1
    kind: Kustomization
    resources:
    - gotk-components.yaml
    - gotk-sync.yaml
    patches:
    - patch: |
        - op: add
          path: /spec/template/spec/containers/0/args/-
          value: --sops-age-secret=sops-age
      target:
        kind: Deployment
        name: kustomize-controller
    ```

## ToDo:

1. Implement small go server application
  1. Expose one url that takes a repo, a username and a commit
  2. Use https://github.com/google/go-github to authenticate as app (read secrets from env)
  3. Check if commit is signed https://github.com/google/go-github and if signed by person in allowed signers
2. Package server app as container
3. Add container app as sidecar to scale_set template
4. Curl to app in hook job started
5. Multiple default routes in podman: https://github.com/containers/podman/issues/23984
6. https://blog.adathor.com/posts/sops-age-yubikey-sb/
7. https://www.windowspro.de/wolfgang-sommergut/dns-abfragen-ueber-https-doh-absichern-windows-10-11
8. https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/about-authentication-with-a-github-app

### Recreate infra stack on single node k8s cluster

- Provision single-node k0s cluster using ignition
  - static ip
  - curl k0sctl to /var/usrlocal/bin
  - write k0sctl.yaml
  - oneshot service to install k0s cluster
  - done?