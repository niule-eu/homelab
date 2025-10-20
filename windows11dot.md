# DNS over TLS on Windows 11

How to manually set DNS in Windows 11:
* https://www.elevenforum.com/t/enable-dns-over-tls-dot-in-windows-11.9012/

How to configure DoT with netsh
``` cmd
netsh dns add global dot=yes
netsh dns add encryption server=172.16.13.2 dothost="dns.infra.ams23.niule.xyz" autoupgrade=yes
```
