output "vpc_id" {
  value = module.vpc.vpc_id
}

output "vswitch_ids" {
  value = module.vpc.vswitch_ids
}

output "slb_external_id" {
  value = module.vpc.slb_external_id
}

output "sg_master_id" {
  value = module.vpc.sg_master_id
}
