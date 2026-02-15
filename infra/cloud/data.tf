data "hcloud_location" "nbg1" {
  name = "nbg1"
}

data "hcloud_image" "fcos_image" {
  with_selector = "os=fedora-coreos,apricote.de/created-by=hcloud-upload-image"
}

data "hcloud_primary_ip" "nbg1-ipv6-1" {
  name = "nbg1-ipv6-1"
}

data "hcloud_server_types" "server_types" {}
