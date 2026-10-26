package live

import (
	"bytes"
	"encoding/json"
	"image"
	"strings"
	"testing"

	"scene-palette/internal/colorx"
	"scene-palette/internal/palette"
)

func TestJointContrastPinnedSurfaces(t *testing.T) {
	request := defaultRequest()
	fresh, err := Build([]image.Image{loadImage(t, "bauhaus-primary.png")}, request)
	if err != nil {
		t.Fatal(err)
	}
	request.Histogram = &fresh.Histogram
	origin := json.RawMessage(`{"kind":"picked","sample":"3x3 mean","repairs":[],"custom":"untouched"}`)
	for _, tc := range []struct {
		name          string
		ground, panel int
		feasible      bool
	}{
		{"black-white feasible", 0, 255, true},
		{"black-near-white feasible", 0, 251, true},
		{"midtones impossible", 89, 170, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request.Core = []palette.SnapshotToken{
				{ID: "ground", Name: "human ground", Job: "floor", R: tc.ground, G: tc.ground, B: tc.ground, Locked: true, Origin: origin},
				{ID: "panel", Name: "human panel", Job: "surface", R: tc.panel, G: tc.panel, B: tc.panel, Locked: true, Origin: origin},
				{ID: "gold", Name: "exact gold", Job: "reserved", R: 100, G: 110, B: 120, Locked: true, Origin: origin},
			}
			result, err := Build(nil, request)
			if err != nil {
				t.Fatal(err)
			}
			for _, plate := range result.Variants {
				for _, index := range []int{0, 1, 7} {
					want := request.Core[0]
					if index == 1 {
						want = request.Core[1]
					}
					if index == 7 {
						want = request.Core[2]
					}
					actual := plate.Core[index]
					if rgb(actual) != rgb(want) || actual.Name != want.Name || actual.Job != want.Job || !bytes.Equal(actual.Origin, want.Origin) {
						t.Fatalf("locked token changed: %+v", actual)
					}
				}
				ground, panel := rgb(plate.Core[0]), rgb(plate.Core[1])
				if tc.feasible {
					for _, token := range plate.Core[2:] {
						if token.Locked {
							continue
						}
						target := 3.0
						if token.ID == "ink" {
							target = 4.5
						}
						if colorx.Contrast(rgb(token), ground) < target || colorx.Contrast(rgb(token), panel) < target {
							t.Fatalf("unlocked %s did not adapt to pinned surfaces", token.ID)
						}
					}
				} else {
					if !strings.Contains(strings.Join(plate.Notes, "\n"), "locked surfaces cannot share readable ink — unlock or adjust a surface") {
						t.Fatalf("impossible pinned surfaces were not disclosed: %v", plate.Notes)
					}
					if rgb(plate.Core[2]) != rgb(fresh.Variants[plate.Params.Variant].Core[2]) {
						t.Fatal("impossible body ink changed despite failed joint repair")
					}
				}
			}
		})
	}
}
