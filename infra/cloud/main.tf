locals {
  niule_authorized_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC9AaghNePDNxI//W6OX6Yp6gssMpDUzK+4XoUiXnRZE fedora-sway-atomic@niuleh.eu"
  lighthouse_hostname  = "lh1.niule.eu"
}

resource "hcloud_ssh_key" "fedora-sway-atomic" {
  name       = "fedora-sway-atomic"
  public_key = local.niule_authorized_key
}

resource "hcloud_primary_ip" "nbg1-ipv4-1" {
  name = "nbg1-ipv4-1"
  location = data.hcloud_location.nbg1.name
  type = "ipv4"
  assignee_type = "server"
  auto_delete = false
}

resource "hcloud_primary_ip" "nbg1-ipv6-2" {
  name = "nbg1-ipv6-2"
  location = data.hcloud_location.nbg1.name
  type = "ipv6"
  assignee_type = "server"
  auto_delete = false
}

resource "hcloud_firewall" "firewall-2" {
  name = "firewall-2"

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = 22
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol = "udp"
    port = 4242
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol = "tcp"
    port = 4242
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }

  rule {
    direction = "in"
    protocol = "icmp"
    source_ips = [
      "0.0.0.0/0",
      "::/0"
    ]
  }
}

resource "hcloud_server" "lh1" {
  name        = "lh1"
  server_type = "cx23"

  ssh_keys = [
    hcloud_ssh_key.fedora-sway-atomic.id
  ]

  location = data.hcloud_location.nbg1.name
  image      = data.hcloud_image.fcos_image.id

  labels = {
    "os" : "fedora-coreos"
  }

  public_net {
    ipv4_enabled = true
    ipv4         = hcloud_primary_ip.nbg1-ipv4-1.id
    ipv6_enabled = true
    ipv6         = data.hcloud_primary_ip.nbg1-ipv6-1.id
  }

  user_data = file("${path.module}/lh1/lh1.ign")
}

resource "hcloud_firewall_attachment" "firewall-2-attachments" {
  firewall_id = hcloud_firewall.firewall-2.id
  server_ids = [
    hcloud_server.lh1.id,
  ]
}