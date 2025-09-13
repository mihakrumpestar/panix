package workflow_definition

type WorkflowPhase string

const (
	PhaseStatus        WorkflowPhase = "status"
	PhasePreBuildHook  WorkflowPhase = "pre-build-hook"
	PhaseBuild         WorkflowPhase = "build"
	PhasePostBuildHook WorkflowPhase = "post-build-hook"
	PhaseBootstrap     WorkflowPhase = "bootstrap"
	PhaseTransfer      WorkflowPhase = "transfer"
	PhaseSecrets       WorkflowPhase = "secrets"
	PhaseActivate      WorkflowPhase = "activate"
	PhaseRollback      WorkflowPhase = "rollback"
)
