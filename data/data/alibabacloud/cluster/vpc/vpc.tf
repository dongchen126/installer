
locals {
  description = "Created By OpenShift Installer"
  prefix      = var.cluster_id
  newbits     = tonumber(split("/", var.vpc_cidr_block)[1]) < 16 ? 20 - tonumber(split("/", var.vpc_cidr_block)[1]) : 4
  resource_group_id = var.ali_resource_group_id == "" ? alicloud_resource_manager_resource_group.resource_group.0.id : var.ali_resource_group_id
  vpc_id = var.vpc_id == null ? alicloud_vpc.vpc.0.id : var.vpc_id
  vswitch_ids = var.vswitch_ids == null ? alicloud_vswitch.vswitchs.*.id : var.vswitch_ids
}

resource "alicloud_vpc" "vpc" {
  count = var.vpc_id == null ? 1 : 0

  resource_group_id = local.resource_group_id
  vpc_name          = "${local.prefix}-vpc"
  cidr_block        = var.vpc_cidr_block
  description       = local.description
  tags = merge(
    {
      "Name" = "${local.prefix}-vpc"
    },
    var.tags,
  )
}

resource "alicloud_vswitch" "vswitchs" {
  count = var.vswitch_ids == null ? length(var.zone_ids) : 0

  vswitch_name = "${local.prefix}-vswitch-${count.index}"
  description  = local.description
  vpc_id       = local.vpc_id
  cidr_block   = cidrsubnet(var.vpc_cidr_block, local.newbits, count.index)
  zone_id      = var.zone_ids[count.index]
  tags = merge(
    {
      "Name" = "${local.prefix}-vswitch"
    },
    var.tags,
  )
}

resource "alicloud_vswitch" "vswitch_nat_gateway" {
  vswitch_name = "${local.prefix}-vswitch-nat-gateway"
  description  = local.description
  vpc_id       = local.vpc_id
  cidr_block   = cidrsubnet(var.vpc_cidr_block, local.newbits, local.newbits)
  zone_id      = var.nat_gateway_zone_id
  tags = merge(
    {
      "Name" = "${local.prefix}-vswitch-nat-gateway"
    },
    var.tags,
  )
}
