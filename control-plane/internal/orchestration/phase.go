package orchestration

import (
	"strings"

	"finalproject/control-plane/internal/model"
)

// MapTrainingPhase 将 Volcano Job 状态与 Pod 相位聚合为业务相位。
func MapTrainingPhase(jobState string, pods []PodSummary) (model.WorkloadPhase, string) {
	switch strings.ToLower(strings.TrimSpace(jobState)) {
	case "completed":
		return model.WorkloadPhaseSucceeded, ""
	case "failed", "aborted", "terminated":
		return model.WorkloadPhaseFailed, jobState
	case "running":
		return model.WorkloadPhaseRunning, ""
	}
	if len(pods) == 0 {
		return model.WorkloadPhasePending, ""
	}
	return mapPodsPhase(pods), ""
}

// MapInferencePhase 根据 Deployment 可用副本与 Pod 相位聚合。
func MapInferencePhase(availableReplicas, desiredReplicas int32, pods []PodSummary) model.WorkloadPhase {
	if desiredReplicas > 0 && availableReplicas >= desiredReplicas {
		allRunning := true
		for _, p := range pods {
			if p.Phase != "Running" && p.Phase != "Succeeded" {
				allRunning = false
				break
			}
		}
		if allRunning && len(pods) > 0 {
			return model.WorkloadPhaseRunning
		}
	}
	if len(pods) == 0 {
		return model.WorkloadPhasePending
	}
	return mapPodsPhase(pods)
}

func mapPodsPhase(pods []PodSummary) model.WorkloadPhase {
	hasRunning := false
	hasPending := false
	allSucceeded := len(pods) > 0
	anyFailed := false
	for _, p := range pods {
		switch p.Phase {
		case "Running":
			hasRunning = true
			allSucceeded = false
		case "Pending":
			hasPending = true
			allSucceeded = false
		case "Succeeded":
			// keep allSucceeded
		case "Failed":
			anyFailed = true
			allSucceeded = false
		default:
			allSucceeded = false
		}
	}
	if anyFailed {
		return model.WorkloadPhaseFailed
	}
	if allSucceeded {
		return model.WorkloadPhaseSucceeded
	}
	if hasRunning {
		return model.WorkloadPhaseRunning
	}
	if hasPending {
		return model.WorkloadPhasePending
	}
	return model.WorkloadPhaseUnknown
}
