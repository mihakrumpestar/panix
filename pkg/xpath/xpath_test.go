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
		{[]string{}, ""},
		{[]string{segA}, Xpath(segA)},
		{[]string{segA, segB}, Xpath(segA + "/" + segB)},
		{[]string{segA, segB, segC}, Xpath(segA + "/" + segB + "/" + segC)},
		{[]string{segA, "", segB}, Xpath(segA + "//" + segB)},
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
		{"", 0},
		{Xpath(segA), 1},
		{Xpath(segA + "/" + segB), 2},
		{Xpath(segA + "/" + segB + "/" + segC), 3},
		{Xpath(segA + "//" + segB), 3},
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
		{"", []string{segA}, Xpath(segA)},
		{"", []string{segA, segB}, Xpath(segA + "/" + segB)},
		{Xpath(segA), []string{segB}, Xpath(segA + "/" + segB)},
		{Xpath(segA), []string{segB, segC}, Xpath(segA + "/" + segB + "/" + segC)},
		{Xpath(segA + "/" + segB + "/" + segC), []string{segD, segE}, Xpath(segA + "/" + segB + "/" + segC + "/" + segD + "/" + segE)},
		{Xpath(segA), []string{""}, Xpath(segA + "/")},
		{"", []string{""}, ""},
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

	fk := faker.New()
	segMach := fk.RandomStringWithLength(4)
	segConf := fk.RandomStringWithLength(4)
	segFlake := fk.RandomStringWithLength(4)
	segExtra := fk.RandomStringWithLength(4)
	segOne := fk.RandomStringWithLength(4)
	segTwo := fk.RandomStringWithLength(4)

	tests := []struct {
		xpath     Xpath
		wantFlake string
		wantConf  string
		wantMach  string
	}{
		{"", "", "", ""},
		{Xpath(segMach), "", "", segMach},
		{Xpath(segConf + "/" + segMach), "", segConf, segMach},
		{Xpath(segFlake + "/" + segConf + "/" + segMach), segFlake, segConf, segMach},
		{Xpath(segExtra + "/" + segFlake + "/" + segConf + "/" + segMach), segFlake, segConf, segMach},
		{Xpath(segOne + "/" + segTwo + "/" + segFlake + "/" + segConf + "/" + segMach), segFlake, segConf, segMach},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			assertion := assert.New(t)
			gotFlake, gotConf, gotMach := tt.xpath.FleetLeaf()
			assertion.Equal(tt.wantFlake, gotFlake)
			assertion.Equal(tt.wantConf, gotConf)
			assertion.Equal(tt.wantMach, gotMach)
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
		{"", ""},
		{Xpath(xpathStr), xpathStr},
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
	assertion.Equal(Xpath(segA+"/"+segB), original)
	assertion.Equal(Xpath(segA+"/"+segB+"/"+segC), appended)
}
