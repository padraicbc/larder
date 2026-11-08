package nutrition

import (
	"context"
	"strings"
	"testing"
)

type productReply string

func (p productReply) Complete(context.Context, string, string) (string, error) {
	return string(p), nil
}

const cheeseReply = `{"name":"Vintage cheddar","ingredients":"Milk, salt, cultures, rennet","hasNutrition":true,"basisUnit":"g","macros":{"calories":410,"protein":25,"carbs":0,"fat":35,"saturatedFat":27,"fiber":0},"notes":"Typical recipe; exact brand may differ.","barcode":"96385074","rating":{"score":100}}`

func TestSuggestProductProvenance(t *testing.T) {
	sc, err := SuggestProduct(context.Background(), productReply(cheeseReply), "Vintage cheddar, Ireland")
	if err != nil || sc.Source != "ai" || sc.Barcode != "" || sc.Rating != nil || sc.BasisQuantity != 100 || sc.Macros.SaturatedFat == nil || *sc.Macros.SaturatedFat != 27 {
		t.Fatalf("suggestion: %+v %v", sc, err)
	}
	r := RateScan(sc)
	if r == nil || r.Score != 62 || !r.Provisional || !strings.Contains(r.ProvisionalReason, "AI estimates") {
		t.Fatalf("rating: %+v", r)
	}
	// A model can abstain from guessing a branded ingredients list.
	sc, err = SuggestProduct(context.Background(), productReply(strings.Replace(cheeseReply, `"Milk, salt, cultures, rennet"`, `""`, 1)), "Unknown brand cheese")
	if err != nil || RateScan(sc) != nil {
		t.Fatal("missing ingredients must remain unrated")
	}
}

func TestSuggestProductRejectsInvalidOutput(t *testing.T) {
	for _, reply := range []string{"not JSON", `{}`, strings.Replace(cheeseReply, `"saturatedFat":27`, `"saturatedFat":36`, 1), strings.Replace(cheeseReply, `"calories":410`, `"calories":1001`, 1), strings.Replace(cheeseReply, `"fat":35`, `"fat":-1`, 1), strings.Replace(cheeseReply, `"basisUnit":"g"`, `"basisUnit":"serving"`, 1)} {
		if _, err := SuggestProduct(context.Background(), productReply(reply), "Cheese"); err == nil {
			t.Fatalf("accepted %s", reply)
		}
	}
	for _, description := range []string{"", strings.Repeat("a", 1001)} {
		if _, err := SuggestProduct(context.Background(), productReply(cheeseReply), description); err == nil {
			t.Fatal("accepted invalid description")
		}
	}
}
