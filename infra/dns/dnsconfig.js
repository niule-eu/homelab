var REG_OVH = NewRegistrar("ovh");
var REG_NONE = NewRegistrar("none");
var DSP_OVH = NewDnsProvider("ovh");

// D("niule.xyz", REG_OVH, DnsProvider(DSP_OVH),
//     DefaultTTL(3600),
//     NAMESERVER_TTL(3600),
//     // NAMESERVER("dns200.anycast.me."),
//     // NAMESERVER("ns200.anycast.me."),
//     A("@", "213.186.33.5"),
//     A("www", "213.186.33.5"),
//     CNAME("ftp", "niule.xyz."),
//     TXT("@", "v=spf1 include:mx.ovh.com ~all"),
//     TXT("@", "1|www.niule.xyz"),
//     TXT("www", "3|welcome"),
//     MX("@", 1, "mx0.mail.ovh.net."),
//     MX("@", 100, "mx3.mail.ovh.net."),
//     MX("@", 5, "mx1.mail.ovh.net."),
//     MX("@", 50, "mx2.mail.ovh.net."),
//     CNAME("ovhmo-selector-1._domainkey", "ovhmo-selector-1._domainkey.4115282.dq.dkim.mail.ovh.net."),
//     CNAME("ovhmo-selector-2._domainkey", "ovhmo-selector-2._domainkey.4115283.dq.dkim.mail.ovh.net."),
// );

D("ams23.niule.xyz", REG_NONE, DnsProvider(DSP_OVH),
    DefaultTTL(3600),
    NAMESERVER_TTL(3600),
    A("@", "213.186.33.5"),
    A("www", "213.186.33.5"),
)
