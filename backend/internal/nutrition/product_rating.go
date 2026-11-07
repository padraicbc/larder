package nutrition

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/padraicbc/larder/internal/models"
)

// HasNutrition reports whether a product has a usable nutrition basis (per 100 g/ml
// or per serving) with at least one value. All-zero entries count as missing.
func HasNutrition(p models.Product) bool {
	unit := p.BasisUnit == "g" || p.BasisUnit == "ml" || p.BasisUnit == "serving"
	m := p.Macros
	return unit && p.BasisQuantity > 0 && (m.Calories > 0 || m.Protein > 0 || m.Carbs > 0 || m.Fat > 0 || m.Fiber > 0)
}

// RateProduct scores a product 0–100 from its ingredients list and macros.
// It is a transparent rule-based heuristic, not medical advice: the
// factors it returns explain every point gained or lost. It returns nil unless
// BOTH the ingredients and the nutrition are known: no rating beats a wrong one.
func RateProduct(p models.Product) *models.ProductRating {
	return RateProductWith(p, models.RatingSettings{})
}

// RateProductWith is RateProduct with one person's tweaks to the rules applied.
func RateProductWith(p models.Product, cfg models.RatingSettings) *models.ProductRating {
	var factors []models.RatingFactor
	rs := &ruleSet{cfg: cfg, add: func(key string, points int, why, format string, a ...any) {
		if points == 0 {
			return // a rule switched off (or a mild one that costs nothing)
		}
		factors = append(factors, models.RatingFactor{Key: key, Text: fmt.Sprintf(format, a...), Points: points, Why: why})
	}}

	// A rating needs both halves. Ingredients alone can't see how energy-dense or
	// protein-rich a food is; nutrition alone can't see additives or added sugar.
	items := splitIngredients(p.Ingredients)
	if len(items) == 0 || !HasNutrition(p) {
		return nil
	}
	rateIngredients(items, rs)
	rs.custom(strings.Join(items, ", "))
	if p.BasisUnit == "g" || p.BasisUnit == "ml" {
		rateNutrition(p.Macros.Scale(100/p.BasisQuantity), rs)
	}

	base := rs.pts("base", ratingBase)
	score := base
	for _, f := range factors {
		score += f.Points
	}
	score = min(max(score, 0), 100)
	sort.SliceStable(factors, func(i, j int) bool { return abs(factors[i].Points) > abs(factors[j].Points) })
	return &models.ProductRating{Rules: RatingRules, At: time.Now().UTC(), Base: base, Score: score, Label: ratingLabel(score), Factors: factors}
}

// ratingBase is where every product starts: a short list of plain ingredients
// stays near the top, and each concern moves it down.
const ratingBase = 90

// RatingRules is bumped whenever the scoring rules change. It is shown with each
// rating so you can tell which rules a given app or server is running.
// v1: first version. v2: missing ingredients penalised and provisional.
// v3: additive count; ingredients read in English first.
// v4: processing markers (yeast extract, cheese powder, dextrose, chicory fibre...) join the additive count.
// v5: no rating at all unless both ingredients and nutrition are known.
const RatingRules = 5

func ratingLabel(score int) string {
	switch {
	case score >= 85:
		return "excellent"
	case score >= 70:
		return "good"
	case score >= 50:
		return "fair"
	case score >= 30:
		return "poor"
	}
	return "avoid"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// splitIngredients splits on top-level commas/semicolons, so
// "chocolate (sugar, cocoa), flour" is two items in label order.
func splitIngredients(s string) []string {
	var out []string
	depth, start := 0, 0
	flush := func(end int) {
		if t := strings.TrimSpace(strings.Trim(s[start:end], " .")); t != "" {
			out = append(out, t)
		}
		start = end + 1
	}
	for i, r := range s {
		switch r {
		case '(', '[':
			depth++
		case ')', ']':
			depth = max(depth-1, 0)
		case ',', ';', '\n', '•':
			if depth == 0 {
				flush(i)
			}
		}
	}
	flush(len(s))
	return out
}

var eNumber = regexp.MustCompile(`(?i)\bE[\s-]?(\d{3,4})([a-z]{1,2}(?:\([iv]+\))?)?\b`)

type additive struct {
	name   string
	points int
}

// Penalties are per distinct additive. Common benign ones (vitamin C, citric
// acid, lecithin, pectin) cost nothing; they are listed so they are recognised.
var eNumbers = map[string]additive{
	"102": {"tartrazine (colour)", -8}, "104": {"quinoline yellow (colour)", -8}, "110": {"sunset yellow (colour)", -8},
	"122": {"carmoisine (colour)", -8}, "124": {"ponceau 4R (colour)", -8}, "129": {"allura red (colour)", -8},
	"171": {"titanium dioxide (colour)", -6}, "150c": {"ammonia caramel (colour)", -3}, "150d": {"sulphite ammonia caramel (colour)", -4},
	"211": {"sodium benzoate (preservative)", -5}, "212": {"potassium benzoate (preservative)", -5},
	"220": {"sulphur dioxide (preservative)", -3}, "223": {"sodium metabisulphite (preservative)", -3},
	"249": {"potassium nitrite (preservative)", -8}, "250": {"sodium nitrite (preservative)", -8},
	"251": {"sodium nitrate (preservative)", -6}, "252": {"potassium nitrate (preservative)", -6},
	"320": {"BHA (preservative)", -8}, "321": {"BHT (preservative)", -8},
	"407": {"carrageenan (thickener)", -3}, "621": {"MSG (flavour enhancer)", -4},
	"627": {"disodium guanylate (flavour enhancer)", -3}, "631": {"disodium inosinate (flavour enhancer)", -3},
	"635": {"disodium ribonucleotides (flavour enhancer)", -3},
	"338": {"phosphoric acid", -3}, "339": {"sodium phosphates", -3}, "340": {"potassium phosphates", -3},
	"450": {"diphosphates", -3}, "451": {"triphosphates", -3}, "452": {"polyphosphates", -3},
	"950": {"acesulfame K (sweetener)", -5}, "951": {"aspartame (sweetener)", -6}, "954": {"saccharin (sweetener)", -5},
	"955": {"sucralose (sweetener)", -5}, "962": {"aspartame-acesulfame salt (sweetener)", -5},
	"433": {"polysorbate 80 (emulsifier)", -4}, "466": {"carboxymethylcellulose (emulsifier)", -4},
	"471": {"mono- and diglycerides (emulsifier)", -2}, "481": {"sodium stearoyl lactylate (emulsifier)", -2},
	"900": {"dimethylpolysiloxane (anti-foam)", -2}, "1422": {"acetylated starch (modified starch)", -2},
	// benign / natural: recognised, not penalised
	"101": {"riboflavin", 0}, "160": {"carotenoid colour", 0}, "162": {"beetroot red", 0}, "163": {"anthocyanins", 0},
	"270": {"lactic acid", 0}, "290": {"carbon dioxide", 0}, "300": {"ascorbic acid (vitamin C)", 0}, "306": {"vitamin E", 0},
	"322": {"lecithin", 0}, "330": {"citric acid", 0}, "331": {"sodium citrates", 0}, "332": {"potassium citrates", 0},
	"334": {"tartaric acid", 0}, "336": {"potassium tartrates", 0},
	"401": {"sodium alginate", 0}, "406": {"agar", 0}, "410": {"locust bean gum", 0}, "412": {"guar gum", 0},
	"415": {"xanthan gum", 0}, "440": {"pectin", 0}, "500": {"sodium bicarbonate", 0}, "501": {"potassium carbonates", 0},
	"503": {"ammonium carbonates", 0}, "509": {"calcium chloride", 0}, "516": {"calcium sulphate", 0}, "170": {"calcium carbonate", 0},
}

// Additives people write out by name instead of by E-number.
var namedAdditives = []struct {
	re     *regexp.Regexp
	name   string
	points int
	why    string
}{
	{regexp.MustCompile(`(?i)\bhigh[- ]fructose|glucose[- ]fructose syrup|fructose[- ]glucose syrup|\bisoglucose`), "high-fructose / glucose-fructose syrup", -8, "A cheap, highly refined sweetener. It adds sugar without any nutrition, and usually signals a sweet, processed product."},
	{regexp.MustCompile(`(?i)partially hydrogenated|\bhydrogenated`), "hydrogenated fat (trans fat risk)", -10, "Hydrogenated fats can contain artificial trans fats, which raise bad cholesterol and are linked to heart disease."},
	{regexp.MustCompile(`(?i)\bmonosodium glutamate|\bmsg\b`), "MSG (flavour enhancer)", -4, "A flavour enhancer that makes food taste more savoury. Generally regarded as safe, but it signals a processed recipe."},
	{regexp.MustCompile(`(?i)\b(aspartame|acesulfame|saccharin|sucralose|cyclamate)\b`), "artificial sweetener", -5, "Sweeteners give sweetness with no sugar. Regulators allow them, but they keep your taste for sweetness high and some people react to them."},
	{regexp.MustCompile(`(?i)\bsodium (nitrite|nitrate)|\bpotassium nitrite`), "nitrite/nitrate preservative", -8, "Used to cure meat. Nitrites can form compounds linked to a higher risk of bowel cancer when processed meat is eaten often."},
	{regexp.MustCompile(`(?i)\b(tartrazine|sunset yellow|carmoisine|ponceau|allura red|quinoline yellow)\b`), "synthetic colour", -8, "Some artificial food colours are linked in studies to hyperactivity in some children, and carry a warning label in the EU."},
	{regexp.MustCompile(`(?i)\bsodium benzoate|\bpotassium sorbate|\bbenzoic acid`), "synthetic preservative", -4, "Preservatives extend shelf life. These are permitted, but can irritate some people and signal a processed product."},
	{regexp.MustCompile(`(?i)\bartificial (flavou?r|colou?r|sweetener)`), "artificial flavour/colour", -5, "Added flavour or colour makes the food look or taste better than its ingredients would on their own."},
	{regexp.MustCompile(`(?i)\b(flavou?r enhancers?|flavou?rings?|natural flavou?rs?|nature[- ]identical)`), "added flavourings", -2, "Flavourings boost taste. They are permitted and mostly signal a more processed recipe, so the cost is small."},
	{regexp.MustCompile(`(?i)\bmaltodextrin|\bmodified (starch|maize|corn|tapioca)`), "maltodextrin / modified starch", -3, "Highly processed starches used as fillers or thickeners. They raise blood sugar quickly and add no nutrients."},
	{regexp.MustCompile(`(?i)\bpalm (oil|fat|kernel)`), "palm oil", -3, "A cheap saturated fat common in processed food. Its production is also linked to deforestation."},
	{regexp.MustCompile(`(?i)\b(emulsifiers?|stabili[sz]ers?|thickeners?|gelling agents?)\b`), "emulsifier/stabiliser", -2, "Added to keep texture smooth and stop ingredients separating. A sign of a processed recipe, and some are being studied for gut effects."},
	{regexp.MustCompile(`(?i)\bmechanically (separated|recovered)`), "mechanically separated meat", -6, "Meat scraped from bones by machine. It is a lower-quality, more processed form of meat."},
	{regexp.MustCompile(`(?i)\bsweeteners?\b`), "sweetener", -3, "Added sweetener, which usually points to a processed product built around sweetness."},
}

var sugarWord = regexp.MustCompile(`(?i)\b(sugar|sucrose|glucose|fructose|dextrose|maltose|syrup|molasses|treacle|invert|caramel|honey|agave|dextrin)\b`)

// ruleSet resolves each rule's points: your override if you set one, else the default.
type ruleSet struct {
	cfg models.RatingSettings
	add func(key string, points int, why, format string, a ...any)
}

func (r *ruleSet) pts(key string, def int) int {
	if v, ok := r.cfg.Points[key]; ok {
		return v
	}
	return def
}

// rule records a factor for key with its default points, unless you changed them.
func (r *ruleSet) rule(key string, def int, why, format string, a ...any) {
	r.add(key, r.pts(key, def), why, format, a...)
}

// custom applies the rules you wrote yourself: points whenever a word or phrase is in the ingredients.
func (r *ruleSet) custom(joined string) {
	for _, c := range r.cfg.Custom {
		m := strings.TrimSpace(c.Match)
		if m == "" || c.Points == 0 {
			continue
		}
		re, err := regexp.Compile(`(?i)(^|[^\p{L}\p{N}])` + regexp.QuoteMeta(m) + `([^\p{L}\p{N}]|$)`)
		if err == nil && re.MatchString(joined) {
			r.add("custom:"+strings.ToLower(m), c.Points, "A rule you added in Settings: ingredients containing “"+m+"” score "+fmt.Sprintf("%+d", c.Points)+".", "“%s” in the ingredients (your rule)", m)
		}
	}
}

func rateIngredients(items []string, rs *ruleSet) {
	joined := strings.Join(items, ", ")

	// E-numbers, deduplicated; letter suffixes (150c) matched exactly first, then the base number.
	seen := map[string]bool{}
	unknown := 0
	for _, m := range eNumber.FindAllStringSubmatch(joined, -1) {
		code := strings.ToLower(m[1] + m[2])
		if i := strings.IndexByte(code, '('); i >= 0 {
			code = code[:i]
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		key := code
		a, ok := eNumbers[key]
		if !ok && len(code) > 3 {
			key = code[:3]
			a, ok = eNumbers[key]
		}
		if !ok {
			key = strings.TrimRight(code, "abcdefghijklmnopqrstuvwxyz")
			a, ok = eNumbers[key]
		}
		if !ok {
			unknown++
			continue
		}
		rs.rule("E"+strings.ToUpper(key), a.points, additiveWhy(a.name), "E%s: %s", strings.ToUpper(code), a.name)
	}
	if unknown > 0 {
		rs.add("unknown-e", rs.pts("unknown-e", -1)*unknown, "Additives with an E-number are worth a look. We have no notes on these ones, so each costs a small amount.", "%d other E-number additive%s", unknown, plural(unknown))
	}

	for _, n := range namedAdditives {
		if n.re.MatchString(joined) {
			rs.rule(n.name, n.points, n.why, "%s", n.name)
		}
	}

	rateAdditiveLoad(joined, rs)

	// Sugars: how many different kinds, and whether one leads the list.
	kinds := map[string]bool{}
	for i, it := range items {
		// don't read "sugar" inside parentheses of a sub-ingredient as top-level, but do count it
		for _, w := range sugarWord.FindAllString(it, -1) {
			kinds[strings.ToLower(w)] = true
		}
		if i < 3 && sugarWord.MatchString(stripParens(it)) {
			rs.rule("sugar-early", -12, "Ingredients are listed from most to least by weight, so sugar near the top means the product is largely sugar.", "Sugar is one of the first three ingredients (%s)", strings.ToLower(stripParens(it)))
			break
		}
	}
	if len(kinds) >= 3 {
		rs.rule("sugar-kinds", -6, "Using several kinds of sugar and syrup spreads them down the list, which can hide how much sugar is really in there.", "%d different sugars/syrups listed", len(kinds))
	}

	if n := len(items); n > 15 {
		rs.rule("long-list-big", -8, "Long lists usually mean a heavily processed product built from refined ingredients and additives, rather than whole foods.", "Very long ingredients list (%d items), typical of ultra-processed food", n)
	} else if n > 10 {
		rs.rule("long-list", -4, "The more ingredients, the more likely the food is processed rather than made from simple whole foods.", "Long ingredients list (%d items)", n)
	} else if n <= 5 {
		rs.rule("short-list", 5, "Few ingredients usually means less processing and nothing hidden.", "Short ingredients list (%d item%s)", n, plural(n))
	}
}

func rateNutrition(per100 models.Macros, rs *ruleSet) {
	switch {
	case per100.Calories > 450:
		rs.rule("energy-high", -8, "Lots of calories in a small weight makes it easy to eat more than you meant to without feeling full.", "Very energy-dense (%.0f kcal per 100)", per100.Calories)
	case per100.Calories > 350:
		rs.rule("energy-mid", -4, "Fairly calorie-heavy for its weight, so portions matter.", "Energy-dense (%.0f kcal per 100)", per100.Calories)
	}
	if per100.Fat > 30 {
		rs.rule("fat-high", -4, "Fat is calorie-dense. We can't see how much is saturated, so this is only a mild flag.", "High in fat (%.0fg per 100)", per100.Fat)
	}
	switch {
	case per100.Fiber >= 6:
		rs.rule("fibre-high", 6, "Fibre helps you feel full and supports gut health, and most people eat too little.", "High in fibre (%.1fg per 100)", per100.Fiber)
	case per100.Fiber >= 3:
		rs.rule("fibre-mid", 3, "Fibre helps you feel full and supports gut health.", "Good source of fibre (%.1fg per 100)", per100.Fiber)
	}
	switch {
	case per100.Protein >= 15:
		rs.rule("protein-high", 5, "Protein helps keep you full and supports muscle.", "High in protein (%.0fg per 100)", per100.Protein)
	case per100.Protein >= 8:
		rs.rule("protein-mid", 2, "Protein helps keep you full and supports muscle.", "Decent protein (%.0fg per 100)", per100.Protein)
	}
}

func stripParens(s string) string {
	if i := strings.IndexAny(s, "(["); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// additiveWhy gives a plain-language reason for an E-number based on its type.
func additiveWhy(name string) string {
	switch {
	case strings.Contains(name, "(colour)"):
		return "Artificial colours add nothing but looks. Some are linked in studies to hyperactivity in some children and carry a warning label in the EU."
	case strings.Contains(name, "(preservative)"):
		return "Preservatives extend shelf life. They are permitted, but some irritate sensitive people, and nitrites in processed meat are linked to higher bowel cancer risk."
	case strings.Contains(name, "(sweetener)"):
		return "Artificial sweeteners give sweetness without sugar. Regulators allow them, but they keep your taste for sweetness high and some people react to them."
	case strings.Contains(name, "(emulsifier)"), strings.Contains(name, "(thickener)"), strings.Contains(name, "(anti-foam)"):
		return "Texture additives that signal a processed recipe. Some emulsifiers are being studied for effects on gut health."
	case strings.Contains(name, "(flavour enhancer)"):
		return "Flavour enhancers make food taste more savoury than its ingredients would, which signals a processed recipe."
	case strings.Contains(name, "phosph"):
		return "Phosphate additives are common in processed food. High intake is a concern for people with kidney problems."
	}
	return "An additive that signals a more processed product. Regulators permit it, so the cost is small."
}

// RateScan rates a shop scan. Nutrition counts only when the scan has it.
func RateScan(sc models.Scan) *models.ProductRating {
	return RateScanWith(sc, models.RatingSettings{})
}

// RateScanWith is RateScan with one person's tweaks to the rules applied.
func RateScanWith(sc models.Scan, cfg models.RatingSettings) *models.ProductRating {
	p := models.Product{Ingredients: sc.Ingredients}
	if sc.HasNutrition {
		p.BasisQuantity, p.BasisUnit, p.Macros = sc.BasisQuantity, sc.BasisUnit, sc.Macros
	}
	r := RateProductWith(p, cfg)
	if r != nil && sc.Warning != "" {
		r.Provisional = true // ingredients present but in another language: some additives may be missed
	}
	return r
}

// ScanFromProduct shows one of your saved products as a scan.
func ScanFromProduct(p models.Product) models.Scan {
	return models.Scan{Name: p.Name, Barcode: p.Barcode, Ingredients: p.Ingredients, HasNutrition: true,
		BasisQuantity: p.BasisQuantity, BasisUnit: p.BasisUnit, Macros: p.Macros, Source: "saved", ProductID: p.ID, Notes: p.Notes, CreatedAt: p.CreatedAt}
}

// ScanFromLabel shows an Open Food Facts result as a scan.
func ScanFromLabel(l LabelEstimate) models.Scan {
	return models.Scan{Name: l.Name, Barcode: l.Barcode, Ingredients: l.Ingredients, HasNutrition: true,
		BasisQuantity: l.BasisQuantity, BasisUnit: l.BasisUnit, Macros: l.Macros, Source: l.Source, Notes: l.Notes, Warning: l.IngredientsWarning}
}

// plainAdditives are common additives that are mild one by one (gums, acids,
// emulsifiers). They don't cost points individually, but they are counted: a long
// additive list is the clearest sign of an industrial, ultra-processed recipe.
var plainAdditives = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`(?i)\bguar gum`), "guar gum"},
	{regexp.MustCompile(`(?i)\bxanthan`), "xanthan gum"},
	{regexp.MustCompile(`(?i)\b(locust bean|carob) gum`), "locust bean gum"},
	{regexp.MustCompile(`(?i)\bgellan`), "gellan gum"},
	{regexp.MustCompile(`(?i)\b(gum arabic|acacia gum|gum acacia)`), "gum arabic"},
	{regexp.MustCompile(`(?i)\bcarrageenan`), "carrageenan"},
	{regexp.MustCompile(`(?i)\bpectin`), "pectin"},
	{regexp.MustCompile(`(?i)\bcitric acid`), "citric acid"},
	{regexp.MustCompile(`(?i)\bascorbic acid`), "ascorbic acid"},
	{regexp.MustCompile(`(?i)\blactic acid`), "lactic acid"},
	{regexp.MustCompile(`(?i)\bmalic acid`), "malic acid"},
	{regexp.MustCompile(`(?i)\blecithin`), "lecithin"},
	{regexp.MustCompile(`(?i)\bmono[- ]?(and|&)?[- ]?di[- ]?glycerides?`), "mono- and diglycerides"},
	{regexp.MustCompile(`(?i)\bsodium (citrate|alginate|carbonate)`), "sodium salts"},
	{regexp.MustCompile(`(?i)\bcalcium (chloride|sulphate|sulfate)`), "calcium salts"},
}

// processingMarkers are ingredients you would not find in a home kitchen. Alongside
// additives they are the usual signs of an ultra-processed (NOVA 4) recipe, so they
// count towards the additive load, though none costs points on its own.
var processingMarkers = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`(?i)\byeast extract`), "yeast extract"},
	{regexp.MustCompile(`(?i)\b(cheese|whey|milk|skimmed milk|skim milk|buttermilk|cream) powder`), "dairy powder"},
	{regexp.MustCompile(`(?i)\b(whey|milk) protein|\bcasein`), "milk protein"},
	{regexp.MustCompile(`(?i)\bdextrose`), "dextrose"},
	{regexp.MustCompile(`(?i)\b(dried )?glucose syrup|\binvert sugar|\bfructose syrup`), "glucose or invert syrup"},
	{regexp.MustCompile(`(?i)\blactose\b`), "lactose"},
	{regexp.MustCompile(`(?i)\binulin|\bchicory (root )?fib(re|er)|\bpolydextrose|\bsoluble (corn |maize )?fib(re|er)`), "added fibre (inulin, chicory, polydextrose)"},
	{regexp.MustCompile(`(?i)\b(soy|soya|pea|wheat|milk|whey) protein (isolate|concentrate)|\bprotein (isolate|concentrate)|\bhydrolysed (vegetable |plant |soy )?protein`), "protein isolate or hydrolysate"},
	{regexp.MustCompile(`(?i)\b(spice|herb|paprika|rosemary|garlic|onion) extracts?`), "spice or herb extract"},
}

// additiveKey makes "Xanthan gum (E415)" and "E415" the same additive.
func additiveKey(name string) string {
	name = strings.ToLower(name)
	if i := strings.Index(name, " ("); i > 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

// countAdditives lists the distinct additives in an ingredients text.
func countAdditives(joined string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if k := additiveKey(name); !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, m := range eNumber.FindAllStringSubmatch(joined, -1) {
		code := strings.ToLower(m[1] + m[2])
		if i := strings.IndexByte(code, '('); i >= 0 {
			code = code[:i]
		}
		if code == "170" || code == "101" { // calcium carbonate (flour fortification) and riboflavin: not counted
			continue
		}
		a, ok := eNumbers[code]
		if !ok && len(code) > 3 {
			a, ok = eNumbers[code[:3]]
		}
		if !ok {
			a, ok = eNumbers[strings.TrimRight(code, "abcdefghijklmnopqrstuvwxyz")]
		}
		if ok {
			add(a.name)
		} else {
			add("e" + code)
		}
	}
	for _, n := range plainAdditives {
		if n.re.MatchString(joined) {
			add(n.name)
		}
	}
	for _, n := range processingMarkers {
		if n.re.MatchString(joined) {
			add(n.name)
		}
	}
	classOnly := 0
	for _, n := range namedAdditives {
		if !n.re.MatchString(joined) {
			continue
		}
		if n.name == "emulsifier/stabiliser" || n.name == "sweetener" { // just a label for the additives themselves
			classOnly++
			continue
		}
		add(n.name)
	}
	if len(out) == 0 && classOnly > 0 {
		add("emulsifier or stabiliser")
	}
	return out
}

// rateAdditiveLoad penalises the number of different additives, however mild each one is.
func rateAdditiveLoad(joined string, rs *ruleSet) {
	names := countAdditives(joined)
	n := len(names)
	key, def := "", 0
	switch {
	case n >= 6:
		key, def = "load-6", -18
	case n >= 4:
		key, def = "load-4", -13
	case n == 3:
		key, def = "load-3", -8
	case n == 2:
		key, def = "load-2", -4
	default:
		return
	}
	list := names
	if len(list) > 6 {
		list = append(append([]string{}, list[:6]...), fmt.Sprintf("and %d more", n-6))
	}
	rs.rule(key, def, "Additives and ingredients you would not use at home (yeast extract, dairy powders, dextrose, added fibre, flavourings, gums, acids) are each mild on their own but add up. A long list of them is the clearest sign of an industrial, ultra-processed (NOVA 4) recipe built for shelf life and taste, not from simple ingredients.",
		"%d additives and processing markers: %s", n, strings.Join(list, ", "))
}
