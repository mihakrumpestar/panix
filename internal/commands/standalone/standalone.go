package commands_standalone

type OutputFlag struct {
	Output string `name:"output" short:"o" help:"Output file path, '-' is stdout" default:"-" completion-predictor:"file"`
}
