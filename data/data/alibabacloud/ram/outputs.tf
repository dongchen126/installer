output "role_master_name" {
  value = alicloud_ram_role.role_master.name
}

output "role_master_arn" {
  value = alicloud_ram_role.role_master.arn
}

output "role_worker_name" {
  value = alicloud_ram_role.role_worker.name
}

output "role_worker_arn" {
  value = alicloud_ram_role.role_worker.arn
}
