variable "vpc_id" {
  type        = string
  description = "The VPC id of the bootstrap ECS."
}

variable "vswitch_id" {
  type        = string
  description = "The VSwitch id of the bootstrap ECS."
}

variable "slb_id" {
  type        = string
  description = "The load balancer of the bootstrap ECS."
}

variable "system_disk_size" {
  type        = number
  description = "The system disk size of the bootstrap ECS."
  default     = 120
}

variable "system_disk_category" {
  type        = string
  description = "The system disk category of the bootstrap ECS.Valid values are cloud_efficiency, cloud_ssd, cloud_essd. Default value is cloud_essd."
  default     = "cloud_essd"
}