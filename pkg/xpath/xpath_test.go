package xpath

import (
	"testing"

	"github.com/jaswdr/faker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	fk := faker.New()
	segA := fk.RandomStringWithLength(4)
	segB := fk.RandomStringWithLength(4)
	segC := fk.RandomStringWithLength(4)

	tests := []struct {
		parts []string
		want  Xpath
	}{
		{[]string{}, Xpath{}},
		{[]string{segA}, New(segA)},
		{[]string{segA, segB}, New(segA, segB)},
		{[]string{segA, segB, segC}, New(segA, segB, segC)},
		{[]string{segA, "", segB}, New(segA, "", segB)},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, New(tt.parts...))
		})
	}
}

func TestDepth(t *testing.T) {
	t.Parallel()

	fk := faker.New()
	segA := fk.RandomStringWithLength(4)
	segB := fk.RandomStringWithLength(4)
	segC := fk.RandomStringWithLength(4)

	tests := []struct {
		xpath Xpath
		want  int
	}{
		{Xpath{}, 0},
		{New(segA), 1},
		{New(segA, segB), 2},
		{New(segA, segB, segC), 3},
		{New(segA, "", segB), 3},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.xpath.Depth())
		})
	}
}

func TestNewXpathWithAppend(t *testing.T) {
	t.Parallel()

	fk := faker.New()
	segA := fk.RandomStringWithLength(4)
	segB := fk.RandomStringWithLength(4)
	segC := fk.RandomStringWithLength(4)
	segD := fk.RandomStringWithLength(4)
	segE := fk.RandomStringWithLength(4)

	tests := []struct {
		base  Xpath
		parts []string
		want  Xpath
	}{
		{Xpath{}, []string{segA}, New(segA)},
		{Xpath{}, []string{segA, segB}, New(segA, segB)},
		{New(segA), []string{segB}, New(segA, segB)},
		{New(segA), []string{segB, segC}, New(segA, segB, segC)},
		{New(segA, segB, segC), []string{segD, segE}, New(segA, segB, segC, segD, segE)},
		{New(segA), []string{""}, New(segA, "")},
		{Xpath{}, []string{""}, New("")},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.base.NewXpathWithAppend(tt.parts...))
		})
	}
}

func TestFleetLeaf(t *testing.T) {
	t.Parallel()

	fake := faker.New()
	segMach := fake.RandomStringWithLength(4)
	segConf := fake.RandomStringWithLength(4)
	segFlake := fake.RandomStringWithLength(4)
	segExtra := fake.RandomStringWithLength(4)
	segOne := fake.RandomStringWithLength(4)
	segTwo := fake.RandomStringWithLength(4)

	tests := []struct {
		xpath     Xpath
		wantFlake string
		wantConf  string
		wantMach  string
	}{
		{Xpath{}, "", "", ""},
		{New(segMach), "", "", segMach},
		{New(segConf, segMach), "", segConf, segMach},
		{New(segFlake, segConf, segMach), segFlake, segConf, segMach},
		{New(segExtra, segFlake, segConf, segMach), segFlake, segConf, segMach},
		{New(segOne, segTwo, segFlake, segConf, segMach), segFlake, segConf, segMach},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			gotFlake, gotConf, gotMach := test.xpath.FleetLeaf()
			assertion.Equal(test.wantFlake, gotFlake)
			assertion.Equal(test.wantConf, gotConf)
			assertion.Equal(test.wantMach, gotMach)
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	fk := faker.New()
	xpathStr := fk.RandomStringWithLength(4) + "/" +
		fk.RandomStringWithLength(4) + "/" +
		fk.RandomStringWithLength(4)

	tests := []struct {
		xpath Xpath
		want  string
	}{
		{Xpath{}, ""},
		{New(xpathStr), xpathStr},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			assertion.Equal(tt.want, tt.xpath.String())
		})
	}
}

func TestImmutability(t *testing.T) {
	t.Parallel()

	fk := faker.New()
	segA := fk.RandomStringWithLength(4)
	segB := fk.RandomStringWithLength(4)
	segC := fk.RandomStringWithLength(4)

	assertion := assert.New(t)
	must := require.New(t)

	original := New(segA, segB)
	appended := original.NewXpathWithAppend(segC)

	must.NotEqual(original, appended)
	assertion.Equal(New(segA, segB), original)
	assertion.Equal(New(segA, segB, segC), appended)
}
