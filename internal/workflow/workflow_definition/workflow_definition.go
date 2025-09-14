package workflow_definition

type WorkflowPhase string

const (
	PhaseStatus        WorkflowPhase = "status"
	PhasePreFlakeHook  WorkflowPhase = "pre-flake-hook"
	PhaseBuild         WorkflowPhase = "build"
	PhaseBootstrap     WorkflowPhase = "bootstrap"
	PhaseTransfer      WorkflowPhase = "transfer"
	PhaseSecrets       WorkflowPhase = "secrets"
	PhaseActivate      WorkflowPhase = "activate"
	PhaseRollback      WorkflowPhase = "rollback"
	PhasePostFlakeHook WorkflowPhase = "post-flake-hook"
)
