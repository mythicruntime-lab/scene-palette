package live

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"scene-palette/internal/colorx"
	"scene-palette/internal/extract"
	"scene-palette/internal/palette"
)

func TestBuildMatchesCompleteBrowserReferencePalettes(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "reference-palettes", "*", "palette.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 10 {
		t.Fatalf("reference palettes = %d; want 10", len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var expected palette.Snapshot
			if err := json.Unmarshal(data, &expected); err != nil {
				t.Fatal(err)
			}
			img := loadImage(t, expected.Sources[0].File)
			req := requestFrom(expected)
			result, err := Build([]image.Image{img}, req)
			if err != nil {
				t.Fatal(err)
			}
			assertJSONEqual(t, result.Variants[expected.Params.Variant], expected)
			req.Histogram = &result.Histogram
			for _, mode := range []extract.Polarity{extract.PolarityAuto, extract.PolarityDark, extract.PolarityLight} {
				req.Polarity = mode
				cached, err := Build(nil, req)
				if err != nil {
					t.Fatalf("cached %s: %v", mode, err)
				}
				req.Histogram = nil
				fresh, err := Build([]image.Image{img}, req)
				if err != nil {
					t.Fatal(err)
				}
				assertJSONEqual(t, cached, fresh)
				req.Histogram = &result.Histogram
			}
		})
	}
}

func TestBuildPreservesLockedRolesAndPins(t *testing.T) {
	req := defaultRequest()
	img := loadImage(t, "bauhaus-primary.png")
	first, err := Build([]image.Image{img}, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Histogram = &first.Histogram
	req.Core = append([]palette.SnapshotToken(nil), first.Variants[0].Core...)
	locked := palette.SnapshotToken{
		ID: "observed", Name: "human ink", Job: "leave untouched", R: 12, G: 34, B: 56, Hex: "#0C2238", Locked: true, Derived: true,
		Origin: json.RawMessage(`{"kind":"picked","still":{"file":"artist.png","hash":{"algo":"sha256","value":"sample"}},"working":{"x":2,"y":3},"natural":{"x":4,"y":6},"method":"3x3 mean","repairs":[],"custom":{"survives":true}}`),
	}
	req.Core[4] = locked
	pin := palette.SnapshotToken{Name: "pinned paint", Job: "hand ink", ID: "custom_pin", R: 200, G: 12, B: 149, Hex: "#C80C95", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)}
	req.Found = []palette.SnapshotToken{pin}
	before, _ := json.Marshal(req)
	for _, mode := range []extract.Polarity{extract.PolarityAuto, extract.PolarityDark, extract.PolarityLight} {
		req.Polarity = mode
		for _, limit := range []int{0, 1, 6} {
			req.Params.FoundInks = limit
			result, err := Build(nil, req)
			if err != nil {
				t.Fatal(err)
			}
			for _, plate := range result.Variants {
				got := plate.Core[4]
				if got.Name != locked.Name || got.Job != locked.Job || got.Hex != locked.Hex || !got.Locked || !got.Derived || !bytes.Equal(got.Origin, locked.Origin) {
					t.Fatalf("locked role changed: %+v", got)
				}
				if len(plate.Found) == 0 || plate.Found[0].Name != pin.Name || plate.Found[0].ID != pin.ID || plate.Found[0].Hex != pin.Hex || !bytes.Equal(plate.Found[0].Origin, pin.Origin) {
					t.Fatalf("pinned ink changed: %+v", plate.Found)
				}
				if limit <= 1 && len(plate.Found) != 1 {
					t.Fatalf("found count %d at limit %d", len(plate.Found), limit)
				}
				data, _ := json.Marshal(plate)
				if _, err := palette.Parse(data); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	req.Polarity = extract.PolarityAuto
	req.Params.FoundInks = 6
	after, _ := json.Marshal(req)
	if !bytes.Equal(before, after) {
		t.Fatal("Build mutated its request")
	}
}

func TestBuildRejectsInvalidInputs(t *testing.T) {
	img := loadImage(t, "bauhaus-primary.png")
	valid := defaultRequest()
	first, err := Build([]image.Image{img}, valid)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*Request)
		images []image.Image
	}{
		{"NaN chroma", func(r *Request) { r.Params.ChromaMin = math.NaN() }, []image.Image{img}},
		{"infinite chroma", func(r *Request) { r.Params.ChromaMin = math.Inf(1) }, []image.Image{img}},
		{"negative budget", func(r *Request) { r.Params.Budget = -1 }, []image.Image{img}},
		{"excessive budget", func(r *Request) { r.Params.Budget = 400001 }, []image.Image{img}},
		{"negative found", func(r *Request) { r.Params.FoundInks = -1 }, []image.Image{img}},
		{"invalid color policy", func(r *Request) { r.Params.ColorPolicy = "other" }, []image.Image{img}},
		{"invalid polarity", func(r *Request) { r.Polarity = "other" }, []image.Image{img}},
		{"invalid time", func(r *Request) { r.CreatedAt = "today" }, []image.Image{img}},
		{"no images", func(r *Request) {}, nil},
		{"nil image", func(r *Request) {}, []image.Image{nil}},
		{"empty image", func(r *Request) {}, []image.Image{image.NewNRGBA(image.Rect(0, 0, 0, 1))}},
		{"too wide", func(r *Request) {}, []image.Image{image.NewNRGBA(image.Rect(0, 0, MaxDimension+1, 1))}},
		{"too many images", func(r *Request) {}, []image.Image{img, img, img, img, img, img, img, img, img}},
		{"too many sources", func(r *Request) { r.Sources = make([]palette.SnapshotSource, 9) }, []image.Image{img}},
		{"source mismatch", func(r *Request) { r.Sources = make([]palette.SnapshotSource, 2) }, []image.Image{img}},
		{"both images and histogram", func(r *Request) { h := first.Histogram; r.Histogram = &h }, []image.Image{img}},
		{"NaN histogram", func(r *Request) { h := first.Histogram; h.Signal[0].A = math.NaN(); r.Histogram = &h }, nil},
		{"negative bin", func(r *Request) { h := first.Histogram; h.Luminance[0].N = -1; r.Histogram = &h }, nil},
		{"inconsistent total", func(r *Request) { h := first.Histogram; h.TotalN += 5; r.Histogram = &h }, nil},
		{"invalid attribution", func(r *Request) { h := first.Histogram; h.BinStills[0] = []int{8}; r.Histogram = &h }, nil},
		{"duplicate attribution", func(r *Request) { h := first.Histogram; h.BinStills[0] = []int{0, 0}; r.Histogram = &h }, nil},
		{"unknown role", func(r *Request) { r.Core = []palette.SnapshotToken{{ID: "unknown"}} }, []image.Image{img}},
		{"duplicate role", func(r *Request) { r.Core = []palette.SnapshotToken{{ID: "ink"}, {ID: "ink"}} }, []image.Image{img}},
		{"invalid RGB", func(r *Request) { r.Core = []palette.SnapshotToken{{ID: "ink", R: 300}} }, []image.Image{img}},
		{"inconsistent hex", func(r *Request) { r.Core = []palette.SnapshotToken{{ID: "ink", Hex: "#FFFFFF"}} }, []image.Image{img}},
		{"invalid origin", func(r *Request) { r.Core = []palette.SnapshotToken{{ID: "ink", Origin: json.RawMessage(`[]`)}} }, []image.Image{img}},
		{"overflowing origin area", func(r *Request) {
			r.Core = []palette.SnapshotToken{{ID: "ink", Origin: json.RawMessage(`{"kind":"imported","area":1e308}`)}}
		}, []image.Image{img}},
		{"tiny weighted color", func(r *Request) {
			h := first.Histogram
			h.Signal[0] = extract.SignalBin{N: 1, WeightSum: 1e-300, L: 1e-7}
			r.Histogram = &h
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)
			if _, err := Build(tc.images, req); err == nil {
				t.Fatal("accepted invalid request")
			}
		})
	}
}

// This uses the actual standalone browser functions as an independent oracle.
// Node is already used by the project's frontend contract checks; no PNG/npm
// decoder is needed because Go supplies the identical decoded pixel bytes.
func TestBuildMatchesBrowserWithLocksAndPins(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is unavailable; run with Node to compare live browser lock behavior")
	}
	img := loadImage(t, "bauhaus-primary.png")
	bounds := img.Bounds()
	pixels := make([]byte, 0, bounds.Dx()*bounds.Dy()*4)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			p := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			pixels = append(pixels, p.R, p.G, p.B, p.A)
		}
	}
	req := defaultRequest()
	first, err := Build([]image.Image{img}, req)
	if err != nil {
		t.Fatal(err)
	}
	req.Core = first.Variants[0].Core
	req.Core[0] = palette.SnapshotToken{ID: "ground", Name: "human ground", Job: "surface", R: 10, G: 11, B: 12, Hex: "#0A0B0C", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)}
	req.Core[4] = palette.SnapshotToken{ID: "observed", Name: "linen", Job: "fixed signal", R: 120, G: 100, B: 60, Hex: "#78643C", Locked: true, Derived: true, Origin: json.RawMessage(`{"kind":"imported","detail":{"preserve":true}}`)}
	req.Found = []palette.SnapshotToken{{Name: "signal", Job: "pinned", R: 12, G: 98, B: 240, Hex: "#0C62F0", Locked: true, Origin: json.RawMessage(`{"kind":"picked","repairs":[]}`)}}
	for _, scenario := range []struct {
		name          string
		ground, panel int
		lockBoth      bool
	}{
		{name: "ground and signal"},
		{name: "narrow feasible pair", ground: 0, panel: 251, lockBoth: true},
		{name: "infeasible pair", ground: 89, panel: 170, lockBoth: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := req
			request.Core = append([]palette.SnapshotToken(nil), req.Core...)
			if scenario.lockBoth {
				for index, value := range []int{scenario.ground, scenario.panel} {
					token := &request.Core[index]
					token.R, token.G, token.B = value, value, value
					token.Hex = colorx.Hex(colorx.RGB{R: value, G: value, B: value})
					token.Locked = true
					token.Origin = json.RawMessage(`{"kind":"picked","sample":"3x3 mean","repairs":[],"custom":{"preserve":true}}`)
				}
			}
			input, err := json.Marshal(map[string]any{"request": request, "width": bounds.Dx(), "height": bounds.Dy(), "pixels": pixels})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(node, "-e", browserOracle, filepath.Join("..", "..", "index.html"))
			command.Stdin = bytes.NewReader(input)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("browser oracle failed: %v\n%s", err, output)
			}
			var expected map[extract.Polarity][]map[string]any
			if err := json.Unmarshal(output, &expected); err != nil {
				t.Fatalf("oracle JSON: %v\n%s", err, output)
			}
			request.Histogram = &first.Histogram
			for _, mode := range []extract.Polarity{extract.PolarityAuto, extract.PolarityDark, extract.PolarityLight} {
				request.Polarity = mode
				result, err := Build(nil, request)
				if err != nil {
					t.Fatal(err)
				}
				actual := make([]map[string]any, 0, len(result.Variants))
				for _, plate := range result.Variants {
					actual = append(actual, map[string]any{"core": browserTokens(plate.Core, false), "found": browserTokens(plate.Found, true), "notes": plate.Notes})
				}
				t.Run(string(mode), func(t *testing.T) { assertJSONEqual(t, actual, expected[mode]) })
			}
		})
	}
}

func browserTokens(tokens []palette.SnapshotToken, found bool) []map[string]any {
	result := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		item := map[string]any{"name": token.Name, "job": token.Job, "r": token.R, "g": token.G, "b": token.B, "hex": token.Hex, "locked": token.Locked, "derived": token.Derived, "origin": token.Origin}
		if !found {
			item["id"] = token.ID
		}
		result = append(result, item)
	}
	return result
}

func defaultRequest() Request {
	return Request{Params: Params{Budget: 140000, ChromaMin: 0.085, FoundInks: 6}, Polarity: extract.PolarityAuto, Name: "Live study", CreatedAt: "2026-08-28T00:00:00.000Z"}
}
func requestFrom(plate palette.Snapshot) Request {
	return Request{Params: Params{ColorPolicy: plate.Params.ColorPolicy, Budget: plate.Params.Budget, ChromaMin: plate.Params.ChromaMin, FoundInks: plate.Params.FoundInks}, Polarity: extract.Polarity(plate.Params.PolarityMode), Name: plate.Name, CreatedAt: plate.CreatedAt, Sources: plate.Sources}
}
func loadImage(t *testing.T, name string) image.Image {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "..", "testdata", "reference-images", name))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return img
}
func assertJSONEqual(t *testing.T, actual, expected any) {
	t.Helper()
	gotBytes, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err = json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON differs\ngot: %s\nwant: %s", gotBytes, wantBytes)
	}
}

var browserOracle = strings.TrimSpace(`
const fs = require('fs'), vm = require('vm');
const input = JSON.parse(fs.readFileSync(0, 'utf8'));
const source = fs.readFileSync(process.argv[1], 'utf8').replaceAll('\r\n', '\n');
function slice(a,b) { const start=source.indexOf(a), end=source.indexOf(b,start); if(start<0||end<0)throw new Error('Missing marker '+a);return source.slice(start,end); }
const controls={quality:{value:String(input.request.params.budget)},chroma:{value:String(input.request.params.chromaMin*1000)},extra:{value:String(input.request.params.foundInks)},extractBtn:{},plateName:{value:'Live study'}};
const errors=[];
const context={console,Uint8ClampedArray,Buffer,location:{protocol:'file:'},$:id=>controls[id],render(){},toast(message,bad){if(bad)errors.push(message)},markProposal(){},plateSourcesFromStills(){return []}};
vm.createContext(context);
const parts=[slice('const clamp =','/* ============================================================\n   3. state'),slice('const QUANTILES =','const $ ='),slice('function histogram(still','/** Scan pixels once'),slice('async function extract()','/** Build structure'),slice('function buildPlate(reset)','/** Auto / Dark / Light'),slice('function applyVariant(i)','\nasync function remix()')];
vm.runInContext(parts.join('\n')+'\nglobalThis.engine={state,extract,buildPlate,applyVariant};',context);
const engine=context.engine;
engine.state.colorPolicy=input.request.params.colorPolicy || "readable-ui";
engine.state.stills=[{w:input.width,h:input.height,_img:new Uint8ClampedArray(Buffer.from(input.pixels,'base64'))}];
function tokens(values,found){return values.map(t=>({...(!found?{id:t.id}:{}),name:t.name,job:t.job,r:t.r,g:t.g,b:t.b,hex:t.hex,locked:!!t.locked,derived:!!t.derived,origin:t.origin}));}
(async()=>{
 await engine.extract();
 const results={};
 for(const mode of ['auto','dark','light']){
  engine.state.polarity=mode;
  engine.state.core=structuredClone(input.request.core || []);
  engine.state.found=structuredClone(input.request.found || []);
  engine.buildPlate(true);
  results[mode]=engine.state.sets.map((_,i)=>{
   engine.state.core=structuredClone(input.request.core || []);
   engine.state.found=structuredClone(input.request.found || []);
   engine.applyVariant(i);
   return {core:tokens(engine.state.core,false),found:tokens(engine.state.found,true),notes:engine.state.notes.slice()};
  });
 }
 if(errors.length)throw new Error(errors.join('\n'));
 process.stdout.write(JSON.stringify(results));
})().catch(error=>{console.error(error);process.exitCode=1});
`)
