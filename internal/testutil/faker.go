package testutil

import (
	"fmt"

	"github.com/jaswdr/faker"

	"github.com/mihakrumpestar/panix/internal/config/attributes"
	"github.com/mihakrumpestar/panix/internal/config/tree/configuration"
	"github.com/mihakrumpestar/panix/internal/config/tree/flake"
	"github.com/mihakrumpestar/panix/internal/config/tree/fleet"
	"github.com/mihakrumpestar/panix/internal/config/tree/machine"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicorderedmap"
	"github.com/mihakrumpestar/panix/internal/pkg/atomic/atomicpointer"
	"github.com/mihakrumpestar/panix/internal/pkg/ssh"
)

const (
	shortStrLen = 4
	homeDirLen  = 6
	secretLen   = 8
)

type Faker struct {
	faker.Faker

	counter int
}

func NewFaker() *Faker {
	return &Faker{faker.New(), 0}
}

func (f *Faker) SSHClient() ssh.SSHClient {
	return ssh.SSHClient{
		Hostname:     f.Internet().Domain(),
		Port:         f.UInt16(),
		Username:     f.Internet().User(),
		IdentityFile: fmt.Sprintf("/home/%s/.ssh/id_%s", f.RandomStringWithLength(homeDirLen), f.RandomStringWithLength(shortStrLen)),
	}
}

func (f *Faker) Machine() *machine.Machine {
	mach := &machine.Machine{
		MetaInspect: atomicpointer.New[machine.MetaInspect](),
		State:       atomicpointer.New[machine.State](),
	}
	mach.SSH = f.SSHClient()
	mach.SudoProgram = attributes.SudoProgram("sudo")

	return mach
}

func (f *Faker) MachineWithSecrets(count int) *machine.Machine {
	mach := f.Machine()

	for range count {
		mach.Secrets = append(mach.Secrets, attributes.PlainFileOrDirToTransfer{
			LocalPath:  fmt.Sprintf("/etc/secrets/%s.key", f.RandomStringWithLength(secretLen)),
			RemotePath: fmt.Sprintf("/etc/secrets/%s.key", f.RandomStringWithLength(secretLen)),
		})
	}

	return mach
}

func (f *Faker) MachineWithBootstrapSSH() *machine.Machine {
	mach := f.Machine()
	mach.Bootstrap.SSH = f.SSHClient()

	return mach
}

func (f *Faker) MachineWithForceBootstrap() *machine.Machine {
	mach := f.Machine()
	mach.Bootstrap.ForceBootstrap = true
	mach.Bootstrap.AllowDestructiveActions = true

	return mach
}

func (f *Faker) Configuration(machines ...*machine.Machine) *configuration.Configuration {
	cfg := &configuration.Configuration{}
	cfg.Machines = atomicorderedmap.New[string, *machine.Machine]()

	for _, mach := range machines {
		cfg.Machines.Set(f.nextID(), mach)
	}

	return cfg
}

func (f *Faker) Flake(configs ...*configuration.Configuration) *flake.Flake {
	flk := &flake.Flake{
		URL: f.Internet().URL(),
	}
	flk.Configurations = atomicorderedmap.New[string, *configuration.Configuration]()

	for _, cfg := range configs {
		flk.Configurations.Set(f.nextID(), cfg)
	}

	return flk
}

func (f *Faker) Fleet(flakes ...*flake.Flake) *fleet.Fleet {
	flt := &fleet.Fleet{}
	flt.Flakes = atomicorderedmap.New[string, *flake.Flake]()

	for _, flk := range flakes {
		flt.Flakes.Set(f.nextID(), flk)
	}

	return flt
}

func (f *Faker) nextID() string {
	f.counter++

	return fmt.Sprintf("id-%d-%s", f.counter, f.RandomStringWithLength(shortStrLen))
}
