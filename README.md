## m720q

### compile ingition and move to file server

``` bash
hlcli render-pkl -m infra/m720q/m720q.pkl | butane | ssh ignition@192.168.3.1 'cat > m720q.ign'
```

### Install to disk

```bash
sudo coreos-installer install /dev/sda --ignition-url http://172.16.13.1:9714/m720q.ign --ignition-hash sha256-78f3efffc8e49779705f5cefa40592314121344eda34c5d33b08435ace796102
```

## Radxa X4

### Create macvlan network

``` bash
podman network create -d macvlan --interface-name eno1 --ipam-driver host-local --subnet 172.16.13.0/24 --gateway 172.16.13.1 -o mode=passthru  macvlan-eno1
```

### DNS deployment

https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/