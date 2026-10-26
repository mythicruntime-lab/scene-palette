package colorx

import "testing"

func TestSourceRepairRetainsContrastAndReducesDistance(t *testing.T) {
	ground, panel := RGB{R: 250, G: 250, B: 250}, RGB{R: 220, G: 220, B: 220}
	improved := false
	for _, raw := range []RGB{{R: 251, G: 245, B: 152}, {R: 205, G: 236, B: 164}, {R: 250, G: 220, B: 245}} {
		legacy, ok := ForceContrastPair(raw, ground, panel, 3)
		if !ok {
			t.Fatal("fixture does not have readable contrast")
		}
		next := CloserContrastPair(raw, legacy, ground, panel, 3, nil, .07)
		if Contrast(next, ground) < 3 || Contrast(next, panel) < 3 {
			t.Fatal("source refinement weakened contrast")
		}
		if DeltaE(raw, next) > DeltaE(raw, legacy) {
			t.Fatal("source refinement moved farther from the same source")
		}
		improved = improved || DeltaE(raw, next) < DeltaE(raw, legacy)
	}
	if !improved {
		t.Fatal("pastel source refinement made no improvement")
	}
}

func TestSourceRepairDoesNotCollapseAnExistingDistinction(t *testing.T) {
	raw := RGB{R: 251, G: 245, B: 152}
	ground, panel := RGB{R: 250, G: 250, B: 250}, RGB{R: 220, G: 220, B: 220}
	current, _ := ForceContrastPair(raw, ground, panel, 3)
	unconstrained := CloserContrastPair(raw, current, ground, panel, 3, nil, .07)
	other := unconstrained
	next := CloserContrastPair(raw, current, ground, panel, 3, []RGB{other}, .07)
	if DeltaE(next, other) < min(.07, DeltaE(current, other)) {
		t.Fatal("source refinement reduced existing role distinction")
	}
}

// These full-precision neutral hues exposed a native Edge cbrt change. They
// must remain exact because sparse source-hue role assignment uses the hue.
func TestReferenceCubeRootNeutralHue(t *testing.T) {
	for _, test := range []struct {
		channel int
		hue     float64
	}{
		{6, 89.87556269699212}, {22, 89.87556266224881}, {23, 89.87556292841498},
	} {
		got := RGBToLCH(RGB{R: test.channel, G: test.channel, B: test.channel}).H
		if got != test.hue {
			t.Fatalf("gray %d hue = %.17g; want %.17g", test.channel, got, test.hue)
		}
	}
}
