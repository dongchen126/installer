resource "alicloud_resource_manager_resource_group" "resource_group" {
  count = var.ali_resource_group_id == "" ? 1 : 0

  resource_group_name = "${var.cluster_id}-rg"
  display_name        = "${var.cluster_id}-rg"
}