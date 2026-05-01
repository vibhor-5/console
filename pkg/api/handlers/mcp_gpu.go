package handlers

import (
	"context"
	"sync"

	"github.com/gofiber/fiber/v2"

	"github.com/kubestellar/console/pkg/k8s"
)

// GetGPUNodes returns nodes with GPU resources
func (h *MCPHandlers) GetGPUNodes(c *fiber.Ctx) error {
	// Demo mode: return demo data immediately
	if isDemoMode(c) {
		return demoResponse(c, "nodes", getDemoGPUNodes())
	}

	cluster := c.Query("cluster")
	if err := mcpValidateName("cluster", cluster); err != nil {
		return err
	}

	if h.k8sClient != nil {
		// If no cluster specified, query all clusters in parallel
		if cluster == "" {
			clusters, _, err := h.k8sClient.HealthyClusters(c.Context())
			if err != nil {
				return handleK8sError(c, err)
			}

			var wg sync.WaitGroup
			var mu sync.Mutex
			allNodes := make([]k8s.GPUNode, 0)
			clusterTimeout := mcpExtendedTimeout // Increased for large GPU clusters
			var errTracker clusterErrorTracker

			clusterCtx, clusterCancel := context.WithCancel(c.Context())
			defer clusterCancel()

			for _, cl := range clusters {
				wg.Add(1)
				go func(clusterName string) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(clusterCtx, clusterTimeout)
					defer cancel()

					nodes, err := h.k8sClient.GetGPUNodes(ctx, clusterName)
					if err != nil {
						errTracker.add(clusterName, err)
					} else if len(nodes) > 0 {
						mu.Lock()
						allNodes = append(allNodes, nodes...)
						mu.Unlock()
					}
				}(cl.Name)
			}

			waitWithDeadline(&wg, clusterCancel, maxResponseDeadline)
			return c.JSON(errTracker.annotate(fiber.Map{"nodes": allNodes, "source": "k8s"}))
		}

		ctx, cancel := context.WithTimeout(c.Context(), mcpExtendedTimeout)
		defer cancel()

		nodes, err := h.k8sClient.GetGPUNodes(ctx, cluster)
		if err != nil {
			return handleK8sError(c, err)
		}
		if nodes == nil {
			nodes = make([]k8s.GPUNode, 0)
		}
		return c.JSON(fiber.Map{"nodes": nodes, "source": "k8s"})
	}

	return errNoClusterAccess(c)
}

// GetGPUNodeHealth returns proactive health check results for GPU nodes
func (h *MCPHandlers) GetGPUNodeHealth(c *fiber.Ctx) error {
	if isDemoMode(c) {
		return demoResponse(c, "nodes", getDemoGPUNodeHealth())
	}

	cluster := c.Query("cluster")
	if err := mcpValidateName("cluster", cluster); err != nil {
		return err
	}

	if h.k8sClient != nil {
		if cluster == "" {
			clusters, _, err := h.k8sClient.HealthyClusters(c.Context())
			if err != nil {
				return handleK8sError(c, err)
			}

			var wg sync.WaitGroup
			var mu sync.Mutex
			allNodes := make([]k8s.GPUNodeHealthStatus, 0)
			clusterTimeout := mcpExtendedTimeout
			var errTracker clusterErrorTracker

			clusterCtx, clusterCancel := context.WithCancel(c.Context())
			defer clusterCancel()

			for _, cl := range clusters {
				wg.Add(1)
				go func(clusterName string) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(clusterCtx, clusterTimeout)
					defer cancel()

					nodes, err := h.k8sClient.GetGPUNodeHealth(ctx, clusterName)
					if err != nil {
						errTracker.add(clusterName, err)
					} else if len(nodes) > 0 {
						mu.Lock()
						allNodes = append(allNodes, nodes...)
						mu.Unlock()
					}
				}(cl.Name)
			}

			waitWithDeadline(&wg, clusterCancel, maxResponseDeadline)
			return c.JSON(errTracker.annotate(fiber.Map{"nodes": allNodes, "source": "k8s"}))
		}

		ctx, cancel := context.WithTimeout(c.Context(), mcpExtendedTimeout)
		defer cancel()

		nodes, err := h.k8sClient.GetGPUNodeHealth(ctx, cluster)
		if err != nil {
			return handleK8sError(c, err)
		}
		if nodes == nil {
			nodes = make([]k8s.GPUNodeHealthStatus, 0)
		}
		return c.JSON(fiber.Map{"nodes": nodes, "source": "k8s"})
	}

	return errNoClusterAccess(c)
}

// GetGPUHealthCronJobStatus returns the installation status of the GPU health CronJob
func (h *MCPHandlers) GetGPUHealthCronJobStatus(c *fiber.Ctx) error {
	if isDemoMode(c) {
		return c.JSON(fiber.Map{"status": k8s.GPUHealthCronJobStatus{CanInstall: true}})
	}

	cluster := c.Query("cluster")
	if cluster == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cluster parameter is required"})
	}
	if err := mcpValidateName("cluster", cluster); err != nil {
		return err
	}

	if h.k8sClient == nil {
		return errNoClusterAccess(c)
	}

	ctx, cancel := context.WithTimeout(c.Context(), mcpDefaultTimeout)
	defer cancel()

	status, err := h.k8sClient.GetGPUHealthCronJobStatus(ctx, cluster)
	if err != nil {
		return handleK8sError(c, err)
	}
	return c.JSON(fiber.Map{"status": status})
}

// InstallGPUHealthCronJob and UninstallGPUHealthCronJob were removed in #7993
// Phase 3e — these user-initiated tooling installs now go through kc-agent at
// `/gpu-health-cronjob` (POST/DELETE), which runs under the user's own
// kubeconfig instead of the backend pod ServiceAccount. The shared
// pkg/k8s.MultiClusterClient.InstallGPUHealthCronJob /
// UninstallGPUHealthCronJob methods stay — kc-agent calls them. See
// pkg/agent/server_gpu_health.go.

// GetGPUHealthCronJobResults returns the latest health check results from the ConfigMap.
// This is the endpoint used by the AlertsContext to evaluate gpu_health_cronjob conditions.
func (h *MCPHandlers) GetGPUHealthCronJobResults(c *fiber.Ctx) error {
	if isDemoMode(c) {
		return c.JSON(fiber.Map{"results": []k8s.GPUHealthCheckResult{}})
	}

	cluster := c.Query("cluster")
	if cluster == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cluster parameter is required"})
	}
	if err := mcpValidateName("cluster", cluster); err != nil {
		return err
	}

	if h.k8sClient == nil {
		return errNoClusterAccess(c)
	}

	ctx, cancel := context.WithTimeout(c.Context(), mcpDefaultTimeout)
	defer cancel()

	status, err := h.k8sClient.GetGPUHealthCronJobStatus(ctx, cluster)
	if err != nil {
		return handleK8sError(c, err)
	}
	return c.JSON(fiber.Map{"results": status.LastResults, "cluster": cluster})
}

// GetNVIDIAOperatorStatus returns NVIDIA GPU and Network operator status
func (h *MCPHandlers) GetNVIDIAOperatorStatus(c *fiber.Ctx) error {
	// Demo mode: return demo data immediately
	if isDemoMode(c) {
		return demoResponse(c, "operators", getDemoNVIDIAOperatorStatus())
	}

	cluster := c.Query("cluster")
	if err := mcpValidateName("cluster", cluster); err != nil {
		return err
	}

	if h.k8sClient != nil {
		// If no cluster specified, query all clusters in parallel
		if cluster == "" {
			clusters, _, err := h.k8sClient.HealthyClusters(c.Context())
			if err != nil {
				return handleK8sError(c, err)
			}

			var wg sync.WaitGroup
			var mu sync.Mutex
			allStatus := make([]*k8s.NVIDIAOperatorStatus, 0)
			clusterTimeout := mcpDefaultTimeout
			var errTracker clusterErrorTracker

			clusterCtx, clusterCancel := context.WithCancel(c.Context())
			defer clusterCancel()

			for _, cl := range clusters {
				wg.Add(1)
				go func(clusterName string) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(clusterCtx, clusterTimeout)
					defer cancel()

					status, err := h.k8sClient.GetNVIDIAOperatorStatus(ctx, clusterName)
					if err != nil {
						errTracker.add(clusterName, err)
					} else if status.GPUOperator != nil || status.NetworkOperator != nil {
						mu.Lock()
						allStatus = append(allStatus, status)
						mu.Unlock()
					}
				}(cl.Name)
			}

			waitWithDeadline(&wg, clusterCancel, maxResponseDeadline)
			return c.JSON(errTracker.annotate(fiber.Map{"operators": allStatus, "source": "k8s"}))
		}

		ctx, cancel := context.WithTimeout(c.Context(), mcpDefaultTimeout)
		defer cancel()

		status, err := h.k8sClient.GetNVIDIAOperatorStatus(ctx, cluster)
		if err != nil {
			return handleK8sError(c, err)
		}
		return c.JSON(fiber.Map{"operators": []*k8s.NVIDIAOperatorStatus{status}, "source": "k8s"})
	}

	return errNoClusterAccess(c)
}
