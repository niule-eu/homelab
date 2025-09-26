## m720q

### compile ingition and move to file server

``` bash
hlcli render-pkl -m infra/m720q/m720q.pkl | butane | ssh ignition@192.168.3.1 'cat > m720q.ign'
```

### Install to disk

```bash
sudo coreos-installer install /dev/nvme0n1 --ignition-url http://172.16.13.1:9714/m720q.ign --ignition-hash sha256-2482978a3aa3acea4a24115f5c3e89f238aed04689ca53f086c54b766e904fce
```