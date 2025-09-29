package phases

type Phase string

const (
	Status        Phase = "status"
	PreFlakeHook  Phase = "pre-flake-hook"
	Build         Phase = "build"
	Bootstrap     Phase = "bootstrap"
	Transfer      Phase = "transfer"
	Secrets       Phase = "secrets"
	Activate      Phase = "activate"
	Rollback      Phase = "rollback"
	PostFlakeHook Phase = "post-flake-hook"
)
