package api

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/padraicbc/larder/internal/models"
)

func oats() models.Product {
	return models.Product{Name: "Oats", Barcode: "96385074", Ingredients: "Wholegrain oats", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true,
		Macros: models.Macros{Calories: 370, Protein: 13, Carbs: 60, Fat: 7, Fiber: 10}}
}

func TestExportOnlyCompleteProducts(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "export@example.com")
	c.do("POST", "/api/products", oats(), nil)
	c.do("POST", "/api/products", models.Product{Name: "Carrots", Generic: true, BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: 41, Protein: 0.9, Carbs: 10, Fiber: 2.8}}, nil)
	// left out: no barcode / no ingredients / no nutrition (and not generic)
	c.do("POST", "/api/products", models.Product{Name: "No code", Ingredients: "Flour", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 300}}, nil)
	c.do("POST", "/api/products", models.Product{Name: "No ingredients", Barcode: "5449000000996", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true, Macros: models.Macros{Calories: 42}}, nil)
	c.do("POST", "/api/products", models.Product{Name: "No nutrition", Barcode: "4006381333931", Ingredients: "Water", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true}, nil)

	resp, err := c.http.Get(c.srv.URL + "/api/products/export")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "larder-products-") || resp.Header.Get("X-Products-Left-Out") != "3" {
		t.Errorf("headers: %v", resp.Header)
	}
	var file productFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range file.Products {
		names = append(names, p.Name)
	}
	if file.Format != "larder-products" || len(file.Products) != 2 || names[0] != "Carrots" && names[1] != "Carrots" {
		t.Fatalf("only Oats and Carrots are complete: %v", names)
	}
	for _, p := range file.Products {
		if p.Name == "Carrots" && (p.Ingredients != "Carrots" || !p.Generic || p.Barcode != "") {
			t.Errorf("a generic food is exported with itself as its ingredient and no barcode: %+v", p)
		}
	}
	// Nothing account-specific or computed leaks into the file.
	for _, banned := range []string{`"rating"`, `"id"`, `"createdAt"`, `"updatedAt"`, "export@example.com"} {
		if strings.Contains(string(raw), banned) {
			t.Errorf("export contains %s", banned)
		}
	}
}

func TestImportNeverOverwritesAndNeedsCompleteProducts(t *testing.T) {
	c0, _ := newTestServer(t)
	a := signUp(t, c0.srv, "from@example.com")
	b := signUp(t, c0.srv, "to@example.com")
	a.do("POST", "/api/products", oats(), nil)
	a.do("POST", "/api/products", models.Product{Name: "Carrots", Generic: true, BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: 41, Protein: 0.9, Carbs: 10, Fiber: 2.8}}, nil)
	var file productFile
	a.do("GET", "/api/products/export", nil, &file)

	// Preview changes nothing.
	var res importResult
	if code := b.do("POST", "/api/products/import/preview", file, &res); code != 200 || len(res.Add) != 2 || res.Applied || len(res.Duplicates)+len(res.Invalid) != 0 {
		t.Fatalf("preview = %d %+v", code, res)
	}
	var mine []models.Product
	b.do("GET", "/api/products", nil, &mine)
	if len(mine) != 0 {
		t.Fatal("a preview must not add anything")
	}
	// Import adds both: generic food keeps no barcode and gets itself as its ingredient.
	res = importResult{}
	if code := b.do("POST", "/api/products/import", file, &res); code != 200 || len(res.Add) != 2 || !res.Applied {
		t.Fatalf("import = %d %+v", code, res)
	}
	b.do("GET", "/api/products", nil, &mine)
	if len(mine) != 2 {
		t.Fatalf("imported: %+v", mine)
	}
	for _, p := range mine {
		if p.Rating == nil || p.ID == "" {
			t.Errorf("imported products are rated and have their own ids: %+v", p)
		}
		if p.Name == "Carrots" && (!p.Generic || p.Barcode != "" || p.Ingredients != "Carrots") {
			t.Errorf("carrots: %+v", p)
		}
	}

	// NO OVERWRITES: change what we have, import the same file again, and nothing is touched.
	var oatsID string
	for _, p := range mine {
		if p.Name == "Oats" {
			oatsID = p.ID
		}
	}
	b.do("PUT", "/api/products/"+oatsID+"/ingredients", map[string]string{"ingredients": "MY OWN EDIT"}, nil)
	res = importResult{}
	b.do("POST", "/api/products/import", file, &res)
	if len(res.Add) != 0 || len(res.Duplicates) != 2 || res.Applied && len(res.Add) != 0 {
		t.Fatalf("a second import should add nothing: %+v", res)
	}
	b.do("GET", "/api/products", nil, &mine)
	if len(mine) != 2 {
		t.Fatalf("no duplicates: %+v", mine)
	}
	for _, p := range mine {
		if p.Name == "Oats" && p.Ingredients != "MY OWN EDIT" {
			t.Errorf("an existing product was overwritten: %+v", p)
		}
	}
	// A different barcode under the same name, and the same barcode under a new name: both are left alone.
	clash := productFile{Format: "larder-products", Version: 1, Products: []fileProduct{
		{Name: "oats", Barcode: "5449000000996", Ingredients: "Other oats", BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: 1}},
		{Name: "Renamed oats", Barcode: "96385074", Ingredients: "Other", BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: 1}},
	}}
	res = importResult{}
	b.do("POST", "/api/products/import", clash, &res)
	if len(res.Add) != 0 || len(res.Duplicates) != 2 || !strings.Contains(res.Duplicates[0].Reason, "name") || !strings.Contains(res.Duplicates[1].Reason, "barcode") {
		t.Fatalf("clashes: %+v", res)
	}
	// The sender's account is untouched by anyone else's import.
	var theirs []models.Product
	a.do("GET", "/api/products", nil, &theirs)
	if len(theirs) != 2 {
		t.Errorf("sender: %+v", theirs)
	}
}

func TestImportRejectsIncompleteProducts(t *testing.T) {
	c0, _ := newTestServer(t)
	c := signUp(t, c0.srv, "bad@example.com")
	good := fileProduct{Name: "Good", Barcode: "96385074", Ingredients: "Oats", BasisQuantity: 100, BasisUnit: "g", Macros: models.Macros{Calories: 370, Protein: 13}}
	with := func(f func(*fileProduct)) fileProduct { p := good; f(&p); return p }
	file := productFile{Format: "larder-products", Version: 1, Products: []fileProduct{
		good,
		with(func(p *fileProduct) { p.Name = "No barcode"; p.Barcode = "" }),
		with(func(p *fileProduct) { p.Name = "Bad check digit"; p.Barcode = "5449000000997" }),
		with(func(p *fileProduct) { p.Name = "No ingredients"; p.Barcode = "5449000000996"; p.Ingredients = "  " }),
		with(func(p *fileProduct) { p.Name = "No basis"; p.Barcode = "4006381333931"; p.BasisQuantity = 0 }),
		with(func(p *fileProduct) { p.Name = "Bad unit"; p.Barcode = "4006381333931"; p.BasisUnit = "cups" }),
		with(func(p *fileProduct) { p.Name = "All zero"; p.Barcode = "4006381333931"; p.Macros = models.Macros{} }),
		with(func(p *fileProduct) { p.Name = "Negative"; p.Barcode = "4006381333931"; p.Macros.Fat = -1 }),
		with(func(p *fileProduct) { p.Name = "   "; p.Barcode = "4006381333931" }),
		with(func(p *fileProduct) { p.Name = "Generic bad barcode"; p.Generic = true; p.Barcode = "123" }),
		with(func(p *fileProduct) {
			p.Name = "Generic no nutrition"
			p.Generic = true
			p.Barcode = ""
			p.Ingredients = ""
			p.Macros = models.Macros{}
		}),
		// allowed: a generic whole food needs no barcode and no ingredients
		with(func(p *fileProduct) {
			p.Name = "Potatoes"
			p.Generic = true
			p.Barcode = ""
			p.Ingredients = ""
			p.Macros = models.Macros{Calories: 77, Protein: 2, Carbs: 17}
		}),
		// repeats within the file
		with(func(p *fileProduct) { p.Name = "Same barcode as Good"; p.Barcode = "96385074" }),
		with(func(p *fileProduct) { p.Name = "good"; p.Barcode = "5449000000996" }),
	}}
	var res importResult
	if code := c.do("POST", "/api/products/import", file, &res); code != 200 {
		t.Fatal(code)
	}
	if len(res.Add) != 2 || res.Add[0].Name != "Good" || res.Add[1].Name != "Potatoes" {
		t.Fatalf("only the complete ones go in: %+v", res.Add)
	}
	reason := map[string]string{}
	for _, p := range res.Invalid {
		reason[p.Name] = p.Reason
	}
	for name, want := range map[string]string{
		"No barcode": "missing barcode", "Bad check digit": "check digit", "No ingredients": "missing ingredients", "No basis": "nutrition basis",
		"Bad unit": "nutrition basis", "All zero": "missing nutrition", "Negative": "zero or more", "Generic bad barcode": "check digit", "Generic no nutrition": "missing nutrition",
	} {
		if !strings.Contains(reason[name], want) {
			t.Errorf("%q should be rejected for %q, got %q", name, want, reason[name])
		}
	}
	if len(res.Invalid) != 10 { // the nine above plus the unnamed one
		t.Errorf("rejected: %+v", res.Invalid)
	}
	if len(res.Duplicates) != 2 || !strings.Contains(res.Duplicates[0].Reason, "barcode") || !strings.Contains(res.Duplicates[1].Reason, "name") {
		t.Errorf("repeats inside the file: %+v", res.Duplicates)
	}
	var mine []models.Product
	c.do("GET", "/api/products", nil, &mine)
	if len(mine) != 2 {
		t.Fatalf("exactly the two valid ones are stored: %+v", mine)
	}

	// Not a products file / too many.
	for _, body := range []any{map[string]any{"format": "larder-backup", "version": 1}, map[string]any{"format": "larder-products", "version": 9}, []int{1}} {
		if code := c.do("POST", "/api/products/import", body, nil); code != 400 {
			t.Errorf("%v = %d", body, code)
		}
	}
	big := productFile{Format: "larder-products", Version: 1}
	for i := 0; i < 2001; i++ {
		big.Products = append(big.Products, fileProduct{Name: fmt.Sprint("p", i)})
	}
	if code := c.do("POST", "/api/products/import/preview", big, nil); code != 400 {
		t.Errorf("too many = %d", code)
	}
}

func TestPortionsSurviveSaveExportImport(t *testing.T) {
	c0, _ := newTestServer(t)
	a := signUp(t, c0.srv, "portions@example.com")
	b := signUp(t, c0.srv, "portions2@example.com")
	var saved models.Product
	a.do("POST", "/api/products", models.Product{Name: "Eggs (boiled)", Generic: true, Source: "usda", BasisQuantity: 100, BasisUnit: "g", FromNutritionLabel: true,
		Macros:   models.Macros{Calories: 155, Protein: 12.6, Carbs: 1.1, Fat: 10.6},
		Portions: []models.Portion{{Label: "1 large", Grams: 50}, {Label: "1 LARGE", Grams: 60}, {Label: "", Grams: 5}, {Label: "1 small", Grams: 38}}}, &saved)
	if len(saved.Portions) != 2 || saved.Portions[0].Label != "1 large" || saved.Portions[0].Grams != 50 || saved.Source != "usda" {
		t.Fatalf("portions are cleaned when saved: %+v", saved)
	}
	var file productFile
	a.do("GET", "/api/products/export", nil, &file)
	if len(file.Products) != 1 || len(file.Products[0].Portions) != 2 || file.Products[0].Source != "usda" {
		t.Fatalf("export keeps portions: %+v", file.Products)
	}
	var res importResult
	b.do("POST", "/api/products/import", file, &res)
	var mine []models.Product
	b.do("GET", "/api/products", nil, &mine)
	if len(mine) != 1 || len(mine[0].Portions) != 2 || mine[0].Portions[1].Label != "1 small" {
		t.Errorf("import keeps portions: %+v", mine)
	}
}
