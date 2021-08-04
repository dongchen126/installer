output "vpc_id" {
  value = module.vpc.vpc_id
}

output "vswitch_ids" {
  value = module.vpc.vswitch_ids
}

output "slb_external_ip" {
  value = module.vpc.slb_external_ip
}

output "slb_internal_ip" {
  value = module.vpc.slb_internal_ip
}

output "sg_master_id" {
  value = module.vpc.sg_master_id
}

output "sg_worker_id" {
  value = module.vpc.sg_worker_id
}