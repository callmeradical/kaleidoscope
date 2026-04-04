package diff

import (
	"testing"

	"github.com/callmeradical/kaleidoscope/snapshot"
)

func rect(x, y, w, h float64) *snapshot.ElementRect {
	return &snapshot.ElementRect{X: x, Y: y, Width: w, Height: h}
}

func elem(role, name string, r *snapshot.ElementRect) snapshot.ElementRecord {
	return snapshot.ElementRecord{Role: role, Name: name, Rect: r}
}

const posT = 4.0
const sizeT = 4.0

func TestElementDiff_Identical(t *testing.T) {
	elements := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
		elem("link", "Home", rect(10, 10, 80, 20)),
	}
	result := ComputeElementDiff(elements, elements, posT, sizeT)

	if result.HasRegression {
		t.Error("expected no regression for identical trees")
	}
	if len(result.Appeared) != 0 {
		t.Errorf("appeared: got %d, want 0", len(result.Appeared))
	}
	if len(result.Disappeared) != 0 {
		t.Errorf("disappeared: got %d, want 0", len(result.Disappeared))
	}
	if len(result.Moved) != 0 {
		t.Errorf("moved: got %d, want 0", len(result.Moved))
	}
	if len(result.Resized) != 0 {
		t.Errorf("resized: got %d, want 0", len(result.Resized))
	}
}

func TestElementDiff_Appeared(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
	}
	snap := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
		elem("dialog", "Confirm", rect(50, 50, 200, 100)),
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if !result.HasRegression {
		t.Error("expected regression when element appeared")
	}
	if len(result.Appeared) != 1 {
		t.Fatalf("appeared: got %d, want 1", len(result.Appeared))
	}
	a := result.Appeared[0]
	if a.Role != "dialog" || a.Name != "Confirm" {
		t.Errorf("appeared element: got {%s,%s}, want {dialog,Confirm}", a.Role, a.Name)
	}
	if a.Snapshot == nil {
		t.Error("appeared element should have Snapshot rect")
	}
}

func TestElementDiff_Disappeared(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
		elem("banner", "Header", rect(0, 0, 1280, 60)),
	}
	snap := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if !result.HasRegression {
		t.Error("expected regression when element disappeared")
	}
	if len(result.Disappeared) != 1 {
		t.Fatalf("disappeared: got %d, want 1", len(result.Disappeared))
	}
	d := result.Disappeared[0]
	if d.Role != "banner" || d.Name != "Header" {
		t.Errorf("disappeared element: got {%s,%s}, want {banner,Header}", d.Role, d.Name)
	}
	if d.Baseline == nil {
		t.Error("disappeared element should have Baseline rect")
	}
}

func TestElementDiff_Moved(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
	}
	snap := []snapshot.ElementRecord{
		elem("button", "Submit", rect(50, 0, 100, 40)), // moved 50px right
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if !result.HasRegression {
		t.Error("expected regression when element moved beyond threshold")
	}
	if len(result.Moved) != 1 {
		t.Fatalf("moved: got %d, want 1", len(result.Moved))
	}
	m := result.Moved[0]
	if m.Delta == nil {
		t.Fatal("moved element should have Delta")
	}
	if m.Delta.DX != 50 {
		t.Errorf("moved DX: got %f, want 50", m.Delta.DX)
	}
}

func TestElementDiff_Resized(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
	}
	snap := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 200, 40)), // widened by 100px
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if !result.HasRegression {
		t.Error("expected regression when element resized beyond threshold")
	}
	if len(result.Resized) != 1 {
		t.Fatalf("resized: got %d, want 1", len(result.Resized))
	}
	r := result.Resized[0]
	if r.Delta == nil {
		t.Fatal("resized element should have Delta")
	}
	if r.Delta.DW != 100 {
		t.Errorf("resized DW: got %f, want 100", r.Delta.DW)
	}
}

func TestElementDiff_BelowThreshold(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("button", "Submit", rect(0, 0, 100, 40)),
	}
	snap := []snapshot.ElementRecord{
		elem("button", "Submit", rect(2, 0, 100, 40)), // moved 2px right — below posT=4
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if result.HasRegression {
		t.Error("expected no regression when position change is below threshold")
	}
	if len(result.Moved) != 0 {
		t.Errorf("moved: got %d, want 0", len(result.Moved))
	}
}

func TestElementDiff_EmptyName(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("img", "", rect(0, 0, 200, 100)),
	}
	snap := []snapshot.ElementRecord{
		elem("img", "", rect(0, 0, 200, 100)),
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if result.HasRegression {
		t.Error("expected no regression for empty-name element with same position")
	}
}

func TestElementDiff_MovedAndResized(t *testing.T) {
	base := []snapshot.ElementRecord{
		elem("button", "Cancel", rect(0, 0, 100, 40)),
	}
	snap := []snapshot.ElementRecord{
		elem("button", "Cancel", rect(50, 50, 200, 80)), // moved and resized
	}
	result := ComputeElementDiff(base, snap, posT, sizeT)

	if len(result.Moved) != 1 {
		t.Errorf("moved: got %d, want 1", len(result.Moved))
	}
	if len(result.Resized) != 1 {
		t.Errorf("resized: got %d, want 1", len(result.Resized))
	}
}

func TestSemanticKey_CaseFoldingAndTrimming(t *testing.T) {
	cases := []struct {
		role, name, want string
	}{
		{" Button ", "", "button:"},
		{"LINK", "Home Page", "link:home page"},
		{"img", "  Logo  ", "img:logo"},
		{"heading", "Title", "heading:title"},
	}
	for _, c := range cases {
		got := SemanticKey(c.role, c.name)
		if got != c.want {
			t.Errorf("SemanticKey(%q,%q) = %q, want %q", c.role, c.name, got, c.want)
		}
	}
}
