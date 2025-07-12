package workflow_definition

type WorkflowPhase string

const (
	PhasePreflight WorkflowPhase = "preflight"
	PhaseBootstrap WorkflowPhase = "bootstrap"
	PhaseSecrets   WorkflowPhase = "secrets"
	PhaseBuild     WorkflowPhase = "build"
	PhaseTransfer  WorkflowPhase = "transfer"
	PhaseActivate  WorkflowPhase = "activate"
	PhaseStatus    WorkflowPhase = "status"
)
