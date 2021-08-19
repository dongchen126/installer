
resource "alicloud_nat_gateway" "nat_gateway" {
  vpc_id           = alicloud_vpc.vpc.id
  specification    = "Small"
  nat_gateway_name = "${local.prefix}-ngw"
  vswitch_id       = alicloud_vswitch.vswitchs[0].id
  nat_type         = "Enhanced"
  description      = local.description
  tags = merge(
    {
      "Name" = "${local.prefix}-ngw"
    },
    var.tags,
  )
}

resource "alicloud_snat_entry" "snat_entry" {
  depends_on        = [alicloud_eip_association.eip_association]
  snat_table_id     = alicloud_nat_gateway.nat_gateway.snat_table_ids
  source_vswitch_id = alicloud_vswitch.vswitchs[0].id
  snat_ip           = alicloud_eip_address.eip.ip_address
}