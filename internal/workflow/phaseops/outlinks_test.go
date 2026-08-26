package phaseops

import (
	"path/filepath"
	"testing"

	"github.com/mihakrumpestar/panix/internal/config/tree/installable"
	"github.com/mihakrumpestar/panix/pkg/xpath"
	"github.com/stretchr/testify/assert"
)

// newOutlinkInstallable returns an installable with only its xpath set.
func newOutlinkInstallable(flake, outputType, name string) *installable.Installable {
	inst := &installable.Installable{}
	inst.Attributes.Xpath = xpath.New(flake, outputType, name)

	return inst
}

// TestOutLinksPaths follows the installable xpath, suffixing the disko
// path, and both are empty when disabled.
func TestOutLinksPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		outLinks    OutLinks
		wantClosure string
		wantDisko   string
	}{
		{"disabled", OutLinks{}, "", ""},
		{
			"default .panix dir",
			OutLinks{Enabled: true, Dir: ".panix"},
			filepath.Join(".panix", "my-flake", "nixosConfigurations", "server1"),
			filepath.Join(".panix", "my-flake", "nixosConfigurations", "server1-disko"),
		},
		{
			"custom dir",
			OutLinks{Enabled: true, Dir: "outroots"},
			filepath.Join("outroots", "my-flake", "nixosConfigurations", "server1"),
			filepath.Join("outroots", "my-flake", "nixosConfigurations", "server1-disko"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			inst := newOutlinkInstallable("my-flake", "nixosConfigurations", "server1")

			assertion := assert.New(t)
			assertion.Equal(test.wantClosure, test.outLinks.ClosurePath(inst))
			assertion.Equal(test.wantDisko, test.outLinks.DiskoPath(inst))
		})
	}
}

// TestOutLinksPaths_MatchesXpath mirrors the installable xpath exactly.
func TestOutLinksPaths_MatchesXpath(t *testing.T) {
	t.Parallel()

	outLinks := OutLinks{Enabled: true, Dir: "roots"}
	inst := newOutlinkInstallable("puntbestanden", "homeConfigurations", "tomvd@boomer")

	assertion := assert.New(t)
	assertion.Equal(filepath.Join("roots", inst.Xpath.String()), outLinks.ClosurePath(inst))
	assertion.Equal(filepath.Join("roots", inst.Xpath.String()+"-disko"), outLinks.DiskoPath(inst))
}
