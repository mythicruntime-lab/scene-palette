package colorx

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestJointContrastMatchesBrowserSolver(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is unavailable; run with Node to compare the browser contrast solver")
	}
	type input struct {
		Color, Ground, Panel [3]int
		Target               float64
	}
	array := func(c RGB) [3]int { return [3]int{c.R, c.G, c.B} }
	colors := []RGB{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {100, 100, 100}, {255, 255, 255}, {}, {60, 100, 180}, {147, 75, 152}, {244, 135, 37}, {12, 98, 240}}
	pairs := [][2]RGB{
		{{19, 18, 32}, {31, 26, 48}},
		{{227, 211, 201}, {206, 188, 177}},
		{{0, 0, 0}, {255, 255, 255}},
		{{0, 0, 0}, {251, 251, 251}}, // 4.5:1 feasible band has no gray.
		{{0, 0, 0}, {170, 170, 170}}, // 3:1 feasible band has no gray.
		{{89, 89, 89}, {170, 170, 170}},
	}
	inputs := []input{}
	for _, color := range colors {
		for _, pair := range pairs {
			for _, target := range []float64{3, 4.5} {
				inputs = append(inputs,
					input{array(color), array(pair[0]), array(pair[1]), target},
					input{array(color), array(pair[1]), array(pair[0]), target})
			}
		}
	}
	data, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	const oracle = `
const fs=require('fs'),vm=require('vm');
const input=JSON.parse(fs.readFileSync(0,'utf8'));
const source=fs.readFileSync(process.argv[1],'utf8').replaceAll('\r\n','\n');
const start=source.indexOf('const clamp ='),end=source.indexOf('/* ============================================================\n   3. state',start);
if(start<0||end<0)throw new Error('Missing color source marker');
const context={};vm.createContext(context);
vm.runInContext(source.slice(start,end)+'\nglobalThis.solve=forceContrastPair;',context);
const rgb=a=>({r:a[0],g:a[1],b:a[2]});
process.stdout.write(JSON.stringify(input.map(c=>{
 const result=context.solve(rgb(c.Color),rgb(c.Ground),rgb(c.Panel),c.Target);
 return {color:[result.color.r,result.color.g,result.color.b],readable:result.readable};
})));`
	command := exec.Command(node, "-e", oracle, filepath.Join("..", "..", "index.html"))
	command.Stdin = bytes.NewReader(data)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("browser contrast oracle: %v\n%s", err, output)
	}
	var expected []struct {
		Color    [3]int `json:"color"`
		Readable bool   `json:"readable"`
	}
	if err := json.Unmarshal(output, &expected); err != nil {
		t.Fatalf("browser contrast oracle JSON: %v\n%s", err, output)
	}
	if len(expected) != len(inputs) {
		t.Fatalf("browser contrast oracle returned %d results; want %d", len(expected), len(inputs))
	}
	rgb := func(a [3]int) RGB { return RGB{a[0], a[1], a[2]} }
	for index, c := range inputs {
		actual, readable := ForceContrastPair(rgb(c.Color), rgb(c.Ground), rgb(c.Panel), c.Target)
		want := expected[index]
		if array(actual) != want.Color || readable != want.Readable {
			t.Errorf("case %+v: Go=%v/%t, browser=%v/%t", c, actual, readable, want.Color, want.Readable)
		}
	}
}

func TestJointContrastKeepsPassingSequentialResults(t *testing.T) {
	colors := []RGB{{0, 0, 0}, {255, 255, 255}, {128, 128, 128}, {147, 75, 152}, {244, 135, 37}, {12, 98, 240}}
	surfaces := [][2]RGB{
		{{19, 18, 32}, {31, 26, 48}},
		{{227, 211, 201}, {206, 188, 177}},
		{{0, 0, 0}, {255, 255, 255}},
	}
	checked := 0
	for _, color := range colors {
		for _, pair := range surfaces {
			for _, target := range []float64{3, 4.5} {
				legacy := ForceContrast(ForceContrast(color, pair[0], target), pair[1], target)
				if Contrast(legacy, pair[0]) < target || Contrast(legacy, pair[1]) < target {
					continue
				}
				actual, ok := ForceContrastPair(color, pair[0], pair[1], target)
				if !ok || actual != legacy {
					t.Fatalf("changed passing sequential repair: %v -> %v; want %v", color, actual, legacy)
				}
				checked++
			}
		}
	}
	if checked < 20 {
		t.Fatalf("insufficient passing repair cases: %d", checked)
	}
}

func TestJointContrastFindsNarrowBlackWhiteBand(t *testing.T) {
	black, white := RGB{}, RGB{255, 255, 255}
	for _, color := range []RGB{{0, 0, 0}, {255, 255, 255}, {0, 200, 80}, {180, 20, 80}, {100, 60, 140}} {
		actual, ok := ForceContrastPair(color, black, white, 4.5)
		if !ok || Contrast(actual, black) < 4.5 || Contrast(actual, white) < 4.5 {
			t.Fatalf("missed feasible black/white body ink for %v: %v (%.8f, %.8f), ok=%t", color, actual, Contrast(actual, black), Contrast(actual, white), ok)
		}
	}
}

func TestJointContrastFindsRepresentableColorWithoutPassingGray(t *testing.T) {
	ground, panel := RGB{}, RGB{251, 251, 251}
	witness := RGB{110, 110, 174}
	if Contrast(witness, ground) < 4.5 || Contrast(witness, panel) < 4.5 {
		t.Fatal("fixture no longer has its known feasible color")
	}
	for gray := 0; gray <= 255; gray++ {
		candidate := RGB{gray, gray, gray}
		if Contrast(candidate, ground) >= 4.5 && Contrast(candidate, panel) >= 4.5 {
			t.Fatal("fixture unexpectedly permits a neutral gray")
		}
	}
	for _, paint := range []RGB{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {100, 100, 100}, {255, 255, 255}, {0, 0, 0}, {60, 100, 180}} {
		actual, ok := ForceContrastPair(paint, ground, panel, 4.5)
		if !ok || Contrast(actual, ground) < 4.5 || Contrast(actual, panel) < 4.5 {
			t.Fatalf("missed feasible 8-bit body ink for %v: %v (%.8f, %.8f), ok=%t", paint, actual, Contrast(actual, ground), Contrast(actual, panel), ok)
		}
	}
}

func TestJointContrastReportsInfeasibleSurfaces(t *testing.T) {
	// Luminances near .1 and .4 have no color that reaches 4.5:1 on both:
	// neither black nor white clears both, and their ratio is below 4.5^2.
	ground, panel, color := RGB{89, 89, 89}, RGB{170, 170, 170}, RGB{180, 70, 110}
	if Contrast(ground, panel) >= 4.5*4.5 {
		t.Fatal("fixture unexpectedly permits an intermediate body ink")
	}
	legacy := ForceContrast(ForceContrast(color, ground, 4.5), panel, 4.5)
	actual, ok := ForceContrastPair(color, ground, panel, 4.5)
	if ok || actual != legacy {
		t.Fatalf("infeasible pair result = %v, %t; want legacy %v, false", actual, ok, legacy)
	}
}

func TestJointContrastNeutralFallbackUsesClosestRepresentableGray(t *testing.T) {
	black, white := RGB{}, RGB{255, 255, 255}
	// For this hue/lightness, the .01 L grid skips the thin shared 4.5:1 band.
	// The fallback must still deliver a valid, deterministic 8-bit neutral.
	var input RGB
	found := false
	for r := 0; r <= 255 && !found; r += 17 {
		candidate := RGB{r, 77, 211}
		actual, ok := ForceContrastPair(candidate, black, white, 4.5)
		if !ok || actual.R != actual.G || actual.G != actual.B {
			continue
		}
		// Independently enumerate representable grays and compare integer distance.
		best, cost := -1, 3*255*255+1
		for gray := 0; gray <= 255; gray++ {
			neutral := RGB{gray, gray, gray}
			if Contrast(neutral, black) < 4.5 || Contrast(neutral, white) < 4.5 {
				continue
			}
			dr, dg, db := gray-candidate.R, gray-candidate.G, gray-candidate.B
			distance := dr*dr + dg*dg + db*db
			if distance < cost {
				best, cost = gray, distance
			}
		}
		if actual.R == best {
			input, found = candidate, true
			if again, ok := ForceContrastPair(input, black, white, 4.5); !ok || again != actual {
				t.Fatal("neutral repair is not deterministic")
			}
		}
	}
	if !found {
		t.Fatal("test inputs did not exercise the neutral fallback")
	}
}
