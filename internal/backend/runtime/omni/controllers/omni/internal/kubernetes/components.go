package kubernetes

import (
    "fmt"

    "github.com/siderolabs/talos/pkg/machinery/constants"
)

// GetComponentImageRefs returns all Kubernetes component image references for a version.
func GetComponentImageRefs(version string) map[string]string {
    return map[string]string{
        "kube-apiserver":          fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, version),
        "kube-controller-manager": fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, version),
        "kube-scheduler":          fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, version),
        "kubelet":                 fmt.Sprintf("%s:v%s", constants.KubeletImage, version),
        "kube-proxy":              fmt.Sprintf("%s:v%s", constants.KubeProxyImage, version),
    }
}