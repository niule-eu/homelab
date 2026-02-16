var REG_NONE = NewRegistrar("none");
var REG_OVH = NewRegistrar("ovh");
var DSP_OVH = NewDnsProvider("ovh");
var DSP_PDNS = NewDnsProvider("powerdns")

D("niule.xyz", REG_OVH, DnsProvider(DSP_OVH),
    DefaultTTL(3600),
    NAMESERVER_TTL(3600),
    // NAMESERVER("dns200.anycast.me."),
    // NAMESERVER("ns200.anycast.me."),
    A("@", "213.186.33.5"),
    A("www", "213.186.33.5"),
    CNAME("ftp", "niule.xyz."),
    CNAME("protonmail._domainkey", "protonmail.domainkey.dvemgfjvchbwbjgduxmmv2d2dtsxgz7ykfazieja4kqtf3m5lz34a.domains.proton.ch."),
    CNAME("protonmail2._domainkey", "protonmail2.domainkey.dvemgfjvchbwbjgduxmmv2d2dtsxgz7ykfazieja4kqtf3m5lz34a.domains.proton.ch."),
    CNAME("protonmail3._domainkey", "protonmail3.domainkey.dvemgfjvchbwbjgduxmmv2d2dtsxgz7ykfazieja4kqtf3m5lz34a.domains.proton.ch."),
    TXT("@", "v=spf1 include:_spf.protonmail.ch ~all"),
    TXT("@", "1|www.niule.xyz"),
    TXT("www", "3|welcome"),
    TXT("@", "protonmail-verification=e5481df0d951f86878c7ec9f8efd9f0efefa6882"),
    TXT("_dmarc", "v=DMARC1; p=quarantine"),
    MX("@", 10, "mail.protonmail.ch."),
    MX("@", 20, "mailsec.protonmail.ch."),
);

D("ams23.niule.xyz", REG_NONE, DnsProvider(DSP_OVH),
    DefaultTTL(3600),
    NAMESERVER_TTL(3600),
    A("@", "213.186.33.5"),
    A("www", "213.186.33.5"),
)

D("infra.ams23.niule.xyz", REG_NONE, DnsProvider(DSP_PDNS), {no_ns: 'true'},
    SOA("@", "ns.infra.ams23.niule.xyz.", "hostmaster.infra.ams23.niule.xyz.", 3600, 600, 604800, 1440),
    A("dns", "172.16.13.2"),
    A("pdnsapi", "172.16.13.2"),
    A("matchbox", "172.16.13.2"),
    A("kea", "172.16.13.3"),
    A("tftp", "172.16.13.3"),
    A("bpir4", "172.16.13.1"),
    IGNORE("", "DHCID"),
    IGNORE("", "A", "172.16.13.{3[1-9],[4-9]?,1??,22[4-9],2[3-5]?}") // Ignores A Records with IPs between 172.16.13.32 - 172.16.13.224 (leaves 172.16.13.0/27 and 172.16.13.224/27 for DHCP)
)
