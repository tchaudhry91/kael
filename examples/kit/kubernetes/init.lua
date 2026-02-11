local M = {}

M.pods_on_node = require("kubernetes.pods_on_node")
M.restart_counts = require("kubernetes.restart_counts")
M.node_capacity = require("kubernetes.node_capacity")
M.pending_pods = require("kubernetes.pending_pods")
M.resource_requests = require("kubernetes.resource_requests")
M.images_in_use = require("kubernetes.images_in_use")

return M
