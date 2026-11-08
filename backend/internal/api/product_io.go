package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/padraicbc/larder/internal/models"
	"github.com/padraicbc/larder/internal/nutrition"
)

const (
	productFileFormat  = "larder-products"
	maxProductFile     = 8 << 20
	maxProductsInFile  = 2000
	maxProductName     = 120
	maxProductIngredes = 4000
)

// productFile is the export format: just the facts about each product. IDs, dates and
// ratings are left out (ratings are worked out when shown), so a file can be imported
// into any account.
type productFile struct {
	Format     string        `json:"format"`
	Version    int           `json:"version"`
	ExportedAt time.Time     `json:"exportedAt"`
	Products   []fileProduct `json:"products"`
}

type fileProduct struct {
	Favourite          bool             `json:"favourite,omitempty"`
	Name               string           `json:"name"`
	Barcode            string           `json:"barcode"`
	Ingredients        string           `json:"ingredients"`
	BasisQuantity      float64          `json:"basisQuantity"`
	BasisUnit          string           `json:"basisUnit"`
	Macros             models.Macros    `json:"macros"`
	FromNutritionLabel *bool            `json:"fromNutritionLabel,omitempty"`
	Confidence         string           `json:"confidence,omitempty"`
	Notes              string           `json:"notes,omitempty"`
	Source             string           `json:"source,omitempty"`
	Generic            bool             `json:"generic,omitempty"`
	Portions           []models.Portion `json:"portions,omitempty"`
}

// exportable: a packaged product is only worth sharing if it can be found (barcode),
// checked (ingredients) and rated (nutrition). A generic whole food (carrots, potatoes)
// has no pack, so it only needs its nutrition.
func exportable(p models.Product) bool {
	if p.Generic {
		return strings.TrimSpace(p.Name) != "" && nutrition.HasNutrition(p)
	}
	return p.Barcode != "" && strings.TrimSpace(p.Ingredients) != "" && nutrition.HasNutrition(p)
}

func (s *Server) exportProducts(w http.ResponseWriter, r *http.Request) {
	all := s.Store.Products(userID(r))
	file := productFile{Format: productFileFormat, Version: 1, ExportedAt: s.Now(), Products: []fileProduct{}}
	for _, p := range all {
		if !exportable(p) {
			continue
		}
		fl := p.FromNutritionLabel
		ing := p.Ingredients
		if p.Generic && strings.TrimSpace(ing) == "" {
			ing = p.Name
		}
		file.Products = append(file.Products, fileProduct{
			Name: p.Name, Favourite: p.Favourite, Barcode: p.Barcode, Ingredients: ing, BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit,
			Macros: p.Macros, FromNutritionLabel: &fl, Confidence: p.Confidence, Notes: p.Notes, Source: p.Source, Generic: p.Generic, Portions: p.Portions,
		})
	}
	w.Header().Set("Content-Disposition", `attachment; filename="larder-products-`+s.Now().Format("2006-01-02")+`.json"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Products-Left-Out", fmt.Sprint(len(all)-len(file.Products)))
	writeJSON(w, 200, file)
}

// importProblem says why one product in the file was not added.
type importProblem struct {
	Name    string `json:"name"`
	Barcode string `json:"barcode,omitempty"`
	Reason  string `json:"reason"`
}

type importItem struct {
	Name    string `json:"name"`
	Barcode string `json:"barcode"`
}

type importResult struct {
	Total      int             `json:"total"`
	Add        []importItem    `json:"add"`        // new products that were (or, in a preview, would be) added
	Duplicates []importProblem `json:"duplicates"` // already in your products: left exactly as they are
	Invalid    []importProblem `json:"invalid"`    // missing a barcode, ingredients or nutrition, or malformed
	Applied    bool            `json:"applied"`
}

// cleanImported checks one product from a file and returns it ready to store, or why not.
func cleanImported(f fileProduct) (models.Product, string) {
	name := strings.Join(strings.Fields(f.Name), " ")
	switch {
	case name == "":
		return models.Product{}, "no name"
	case len([]rune(name)) > maxProductName:
		return models.Product{}, "name is too long"
	}
	code := ""
	if strings.TrimSpace(f.Barcode) != "" {
		var ok bool
		if code, ok = nutrition.NormalizeBarcode(f.Barcode); !ok {
			return models.Product{}, "barcode isn't valid (check digit fails)"
		}
	} else if !f.Generic {
		return models.Product{}, "missing barcode (a generic whole food like carrots doesn't need one)"
	}
	ing := strings.TrimSpace(f.Ingredients)
	if ing == "" && f.Generic {
		ing = name // whole food: the food is its own ingredient
	}
	switch {
	case ing == "":
		return models.Product{}, "missing ingredients"
	case len(ing) > maxProductIngredes:
		return models.Product{}, "ingredients are too long"
	}
	unit := strings.ToLower(strings.TrimSpace(f.BasisUnit))
	if !(f.BasisQuantity > 0) || math.IsInf(f.BasisQuantity, 0) || (unit != "g" && unit != "ml" && unit != "serving") {
		return models.Product{}, "missing or invalid nutrition basis (an amount in g, ml or servings)"
	}
	m := f.Macros
	if !m.ValidSaturatedFat() {
		return models.Product{}, "saturated fat must be between zero and total fat"
	}
	for _, v := range []float64{m.Calories, m.Protein, m.Carbs, m.Fat, m.Fiber} {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return models.Product{}, "nutrition values must be zero or more"
		}
	}
	p := models.Product{Favourite: f.Favourite, Name: name, Barcode: code, Ingredients: ing, BasisQuantity: f.BasisQuantity, BasisUnit: unit, Macros: m, Generic: f.Generic, Portions: nutrition.CleanPortions(f.Portions)}
	if !nutrition.HasNutrition(p) {
		return models.Product{}, "missing nutrition values"
	}
	p.FromNutritionLabel = f.FromNutritionLabel == nil || *f.FromNutritionLabel
	p.Confidence = "medium"
	if c := strings.ToLower(f.Confidence); c == "low" || c == "medium" || c == "high" {
		p.Confidence = c
	}
	if !p.FromNutritionLabel {
		p.Confidence = "low"
	}
	if n := strings.TrimSpace(f.Notes); len(n) <= 500 {
		p.Notes = n
	} else {
		p.Notes = n[:500]
	}
	if f.Source == "openfoodfacts" || f.Source == "usda" || f.Source == "ai" {
		p.Source = f.Source
	}
	return p, ""
}

// planImport sorts a file into products to add, products you already have, and rejects.
// It changes nothing, so the preview and the real import follow the same rules.
func planImport(existing []models.Product, file productFile) ([]models.Product, importResult) {
	res := importResult{Total: len(file.Products), Add: []importItem{}, Duplicates: []importProblem{}, Invalid: []importProblem{}}
	haveBarcode, haveName := map[string]string{}, map[string]bool{}
	for _, e := range existing {
		if e.Barcode != "" {
			haveBarcode[e.Barcode] = e.Name
		}
		haveName[strings.ToLower(strings.TrimSpace(e.Name))] = true
	}
	var add []models.Product
	for i, f := range file.Products {
		label := strings.TrimSpace(f.Name)
		if label == "" {
			label = fmt.Sprintf("(item %d, no name)", i+1)
		}
		p, why := cleanImported(f)
		if why != "" {
			res.Invalid = append(res.Invalid, importProblem{Name: label, Barcode: strings.TrimSpace(f.Barcode), Reason: why})
			continue
		}
		key := strings.ToLower(p.Name)
		if other, dup := haveBarcode[p.Barcode]; dup && p.Barcode != "" {
			res.Duplicates = append(res.Duplicates, importProblem{Name: p.Name, Barcode: p.Barcode, Reason: "you already have this barcode as “" + other + "”"})
			continue
		}
		if haveName[key] {
			res.Duplicates = append(res.Duplicates, importProblem{Name: p.Name, Barcode: p.Barcode, Reason: "you already have a product with this name"})
			continue
		}
		if p.Barcode != "" {
			haveBarcode[p.Barcode] = p.Name
		}
		haveName[key] = true // later rows in the same file can't repeat it either
		add = append(add, p)
		res.Add = append(res.Add, importItem{Name: p.Name, Barcode: p.Barcode})
	}
	return add, res
}

// importProducts handles both /import/preview (changes nothing) and /import.
func (s *Server) importProducts(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxProductFile)
	var file productFile
	if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
		writeErr(w, 400, "this isn't a Larder products file: "+err.Error())
		return
	}
	if file.Format != productFileFormat || file.Version != 1 {
		writeErr(w, 400, "this isn't a Larder products file (export one from Products first)")
		return
	}
	if len(file.Products) > maxProductsInFile {
		writeErr(w, 400, fmt.Sprintf("that file has more than %d products", maxProductsInFile))
		return
	}
	uid := userID(r)
	toAdd, res := planImport(s.Store.Products(uid), file)
	if r.URL.Path == "/api/products/import/preview" || len(toAdd) == 0 {
		writeJSON(w, 200, res)
		return
	}
	added, skipped, err := s.Store.AddProducts(uid, toAdd, s.Now())
	if err != nil {
		storeErr(w, err)
		return
	}
	// The store re-checks under its lock; anything it refused moves from "add" to "duplicates".
	if len(skipped) > 0 {
		kept := map[string]bool{}
		for _, p := range added {
			kept[strings.ToLower(p.Name)] = true
		}
		var still []importItem
		for _, it := range res.Add {
			if kept[strings.ToLower(it.Name)] {
				still = append(still, it)
			} else {
				res.Duplicates = append(res.Duplicates, importProblem{Name: it.Name, Barcode: it.Barcode, Reason: "added by someone else a moment ago"})
			}
		}
		res.Add = still
	}
	res.Applied = true
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, res)
}
