output "bootstrap_ecs_ip" {
  value = data.alicloud_instances.bootstrap_data.instances.0.private_ip
}
