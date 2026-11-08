#!/usr/bin/env python3
"""Regenerates backend/internal/nutrition/foods.json (the built-in everyday foods).

The numbers are copied from USDA FoodData Central records, never typed by hand. To add a
food, add a line to FOODS below with the exact USDA description, then re-run:

  1. Download "SR Legacy" (CSV) from https://fdc.nal.usda.gov/download-datasets and unzip it
     (the data is public domain / CC0).
  2. python3 -I scripts/build_foods.py <unzipped-folder> backend/internal/nutrition/foods.json
  3. cd backend && go test ./internal/nutrition   (checks every value is sane)

The script stops if a description isn't found, so a typo can't slip in a made-up food.
"""
import csv, json, sys
data, out = sys.argv[1], sys.argv[2]

# friendly name, search aliases, category, exact USDA SR Legacy description
FOODS = [
 # --- potatoes & roots
 ("Potatoes (white)", ["potato", "potatoes", "spuds", "baby potatoes", "new potatoes"], "Vegetables", "Potatoes, white, flesh and skin, raw"),
 ("Potatoes (red)", ["red potato", "red potatoes"], "Vegetables", "Potatoes, red, flesh and skin, raw"),
 ("Potatoes (baking / russet)", ["baking potato", "russet", "maris piper", "rooster"], "Vegetables", "Potatoes, russet, flesh and skin, raw (Includes foods for USDA's Food Distribution Program)"),
 ("Sweet potatoes", ["sweet potato"], "Vegetables", "Sweet potato, raw, unprepared (Includes foods for USDA's Food Distribution Program)"),
 ("Carrots", ["carrot"], "Vegetables", "Carrots, raw"),
 ("Parsnips", ["parsnip"], "Vegetables", "Parsnips, raw"),
 ("Turnips", ["turnip"], "Vegetables", "Turnips, raw"),
 ("Swede", ["rutabaga", "turnip swede"], "Vegetables", "Rutabagas, raw"),
 ("Beetroot", ["beets", "beet"], "Vegetables", "Beets, raw"),
 ("Radishes", ["radish"], "Vegetables", "Radishes, raw"),
 # --- salad & leaves
 ("Tomatoes", ["tomato", "cherry tomatoes", "plum tomatoes", "vine tomatoes", "salad tomatoes"], "Vegetables", "Tomatoes, red, ripe, raw, year round average"),
 ("Cucumber", ["cucumbers"], "Vegetables", "Cucumber, with peel, raw"),
 ("Lettuce (iceberg)", ["iceberg"], "Vegetables", "Lettuce, iceberg (includes crisphead types), raw"),
 ("Lettuce (cos / romaine)", ["romaine", "cos", "little gem"], "Vegetables", "Lettuce, cos or romaine, raw"),
 ("Lettuce (green leaf)", ["salad leaves", "green lettuce"], "Vegetables", "Lettuce, green leaf, raw"),
 ("Lettuce (butterhead)", ["butterhead"], "Vegetables", "Lettuce, butterhead (includes boston and bibb types), raw"),
 ("Spinach", ["baby spinach"], "Vegetables", "Spinach, raw"),
 ("Rocket", ["arugula"], "Vegetables", "Arugula, raw"),
 ("Kale", [], "Vegetables", "Kale, raw"),
 ("Celery", [], "Vegetables", "Celery, raw"),
 # --- brassicas
 ("Broccoli", [], "Vegetables", "Broccoli, raw"),
 ("Cauliflower", [], "Vegetables", "Cauliflower, raw"),
 ("Cabbage (white / green)", ["cabbage"], "Vegetables", "Cabbage, raw"),
 ("Cabbage (red)", ["red cabbage"], "Vegetables", "Cabbage, red, raw"),
 ("Cabbage (savoy)", ["savoy"], "Vegetables", "Cabbage, savoy, raw"),
 ("Brussels sprouts", ["sprouts"], "Vegetables", "Brussels sprouts, raw"),
 # --- onion family
 ("Onions", ["onion", "red onion", "brown onion", "white onion"], "Vegetables", "Onions, raw"),
 ("Spring onions", ["scallions", "scallion", "green onions"], "Vegetables", "Onions, spring or scallions (includes tops and bulb), raw"),
 ("Leeks", ["leek"], "Vegetables", "Leeks, (bulb and lower leaf-portion), raw"),
 ("Garlic", [], "Vegetables", "Garlic, raw"),
 # --- peppers, squash, others
 ("Peppers (red)", ["red pepper", "bell pepper"], "Vegetables", "Peppers, sweet, red, raw"),
 ("Peppers (green)", ["green pepper", "bell pepper"], "Vegetables", "Peppers, sweet, green, raw"),
 ("Peppers (yellow)", ["yellow pepper", "bell pepper"], "Vegetables", "Peppers, sweet, yellow, raw"),
 ("Mushrooms (white)", ["mushroom", "button mushrooms", "closed cup mushrooms"], "Vegetables", "Mushrooms, white, raw"),
 ("Mushrooms (portobello)", ["portabella", "portobello", "chestnut mushrooms"], "Vegetables", "Mushrooms, portabella, raw"),
 ("Courgette", ["zucchini", "courgettes"], "Vegetables", "Squash, summer, zucchini, includes skin, raw"),
 ("Aubergine", ["eggplant"], "Vegetables", "Eggplant, raw"),
 ("Butternut squash", ["squash"], "Vegetables", "Squash, winter, butternut, raw"),
 ("Pumpkin", [], "Vegetables", "Pumpkin, raw"),
 ("Green beans", ["snap beans", "french beans", "runner beans"], "Vegetables", "Beans, snap, green, raw"),
 ("Peas (green)", ["garden peas", "petits pois"], "Vegetables", "Peas, green, raw"),
 ("Sweetcorn", ["corn", "corn on the cob"], "Vegetables", "Corn, sweet, yellow, raw"),
 ("Asparagus", [], "Vegetables", "Asparagus, raw"),
 ("Ginger", ["root ginger", "ginger root"], "Vegetables", "Ginger root, raw"),
 # --- fruit
 ("Apples", ["apple", "gala", "braeburn", "granny smith", "fuji", "pink lady"], "Fruit", "Apples, raw, with skin (Includes foods for USDA's Food Distribution Program)"),
 ("Bananas", ["banana"], "Fruit", "Bananas, raw"),
 ("Grapes", ["grape", "red grapes", "green grapes", "seedless grapes"], "Fruit", "Grapes, red or green (European type, such as Thompson seedless), raw"),
 ("Oranges", ["orange", "navel orange"], "Fruit", "Oranges, raw, all commercial varieties"),
 ("Clementines / mandarins", ["tangerines", "satsumas", "easy peelers", "clementine", "mandarin"], "Fruit", "Tangerines, (mandarin oranges), raw"),
 ("Grapefruit", [], "Fruit", "Grapefruit, raw, pink and red, all areas"),
 ("Lemons", ["lemon"], "Fruit", "Lemons, raw, without peel"),
 ("Strawberries", ["strawberry"], "Fruit", "Strawberries, raw"),
 ("Blueberries", ["blueberry"], "Fruit", "Blueberries, raw"),
 ("Raspberries", ["raspberry"], "Fruit", "Raspberries, raw"),
 ("Blackberries", ["blackberry"], "Fruit", "Blackberries, raw"),
 ("Pears", ["pear", "conference pears"], "Fruit", "Pears, raw"),
 ("Peaches", ["peach"], "Fruit", "Peaches, yellow, raw"),
 ("Nectarines", ["nectarine"], "Fruit", "Nectarines, raw"),
 ("Plums", ["plum"], "Fruit", "Plums, raw"),
 ("Apricots", ["apricot"], "Fruit", "Apricots, raw"),
 ("Cherries", ["cherry"], "Fruit", "Cherries, sweet, raw"),
 ("Kiwi fruit", ["kiwi", "kiwifruit"], "Fruit", "Kiwifruit, green, raw"),
 ("Pineapple", [], "Fruit", "Pineapple, raw, all varieties"),
 ("Melon (cantaloupe)", ["cantaloupe", "melon"], "Fruit", "Melons, cantaloupe, raw"),
 ("Watermelon", [], "Fruit", "Watermelon, raw"),
 ("Mango", ["mangos", "mangoes"], "Fruit", "Mangos, raw"),
 ("Avocado", ["avocados"], "Fruit", "Avocados, raw, all commercial varieties"),
 ("Pomegranate", ["pomegranates"], "Fruit", "Pomegranates, raw"),
 ("Figs", ["fig"], "Fruit", "Figs, raw"),
]

# eggs, meat and poultry. Raw and cooked are separate because people weigh them either way.
FOODS += [
 ("Eggs (raw)", ["egg", "eggs"], "Eggs", "Egg, whole, raw, fresh"),
 ("Eggs (boiled)", ["egg", "eggs", "boiled egg", "hard boiled"], "Eggs", "Egg, whole, cooked, hard-boiled"),
 ("Eggs (poached)", ["egg", "eggs", "poached egg"], "Eggs", "Egg, whole, cooked, poached"),
 ("Eggs (fried)", ["egg", "eggs", "fried egg"], "Eggs", "Egg, whole, cooked, fried"),
 ("Eggs (scrambled)", ["egg", "eggs", "scrambled egg"], "Eggs", "Egg, whole, cooked, scrambled"),
 ("Omelette", ["omelet", "egg", "eggs"], "Eggs", "Egg, whole, cooked, omelet"),
 ("Chicken breast (raw)", ["chicken", "chicken fillet", "chicken breast"], "Meat & poultry", "Chicken, broiler or fryers, breast, skinless, boneless, meat only, raw"),
 ("Chicken breast (cooked)", ["chicken", "roast chicken breast", "chicken breast"], "Meat & poultry", "Chicken, broilers or fryers, breast, meat only, cooked, roasted"),
 ("Chicken thigh (raw)", ["chicken", "chicken thighs"], "Meat & poultry", "Chicken, broilers or fryers, dark meat, thigh, meat only, raw"),
 ("Chicken thigh (cooked)", ["chicken", "chicken thighs"], "Meat & poultry", "Chicken, broilers or fryers, thigh, meat only, cooked, roasted"),
 ("Chicken with skin (raw)", ["chicken", "whole chicken"], "Meat & poultry", "Chicken, broilers or fryers, meat and skin, raw"),
 ("Chicken with skin (roast)", ["chicken", "roast chicken", "whole chicken"], "Meat & poultry", "Chicken, broilers or fryers, meat and skin, cooked, roasted"),
 ("Chicken mince (raw)", ["chicken", "ground chicken"], "Meat & poultry", "Chicken, ground, raw"),
 ("Turkey breast (raw)", ["turkey"], "Meat & poultry", "Turkey, retail parts, breast, meat only, raw"),
 ("Turkey breast (cooked)", ["turkey", "roast turkey"], "Meat & poultry", "Turkey, retail parts, breast, meat only, cooked, roasted"),
 ("Turkey mince (raw)", ["turkey", "ground turkey"], "Meat & poultry", "Turkey, ground, 93% lean, 7% fat, raw"),
 ("Beef mince 5% fat (raw)", ["beef", "mince", "ground beef", "lean mince"], "Meat & poultry", "Beef, ground, 95% lean meat / 5% fat, raw"),
 ("Beef mince 5% fat (cooked)", ["beef", "mince", "ground beef", "lean mince"], "Meat & poultry", "Beef, ground, 95% lean meat / 5% fat, crumbles, cooked, pan-browned"),
 ("Beef mince 10% fat (raw)", ["beef", "mince", "ground beef"], "Meat & poultry", "Beef, ground, 90% lean meat / 10% fat, raw"),
 ("Beef mince 10% fat (cooked)", ["beef", "mince", "ground beef"], "Meat & poultry", "Beef, ground, 90% lean meat / 10% fat, crumbles, cooked, pan-browned"),
 ("Beef mince 20% fat (raw)", ["beef", "mince", "ground beef"], "Meat & poultry", "Beef, ground, 80% lean meat / 20% fat, raw"),
 ("Beef mince 20% fat (cooked)", ["beef", "mince", "ground beef"], "Meat & poultry", "Beef, ground, 80% lean meat / 20% fat, crumbles, cooked, pan-browned"),
 ("Sirloin steak (raw)", ["beef", "steak"], "Meat & poultry", "Beef, top sirloin, steak, separable lean only, trimmed to 1/8\" fat, choice, raw"),
 ("Sirloin steak (grilled)", ["beef", "steak"], "Meat & poultry", "Beef, top sirloin, steak, separable lean only, trimmed to 1/8\" fat, choice, cooked, broiled"),
 ("Ribeye steak (raw)", ["beef", "steak", "rib eye"], "Meat & poultry", "Beef, rib eye steak, boneless, lip off, separable lean only, trimmed to 0\" fat, choice, raw"),
 ("Ribeye steak (grilled)", ["beef", "steak", "rib eye"], "Meat & poultry", "Beef, rib eye steak, boneless, lip off, separable lean only, trimmed to 0\" fat, choice, cooked, grilled"),
 ("Pork mince (raw)", ["pork", "ground pork"], "Meat & poultry", "Pork, fresh, ground, raw"),
 ("Pork mince (cooked)", ["pork", "ground pork"], "Meat & poultry", "Pork, fresh, ground, cooked"),
 ("Pork chop (raw)", ["pork", "pork loin", "chop"], "Meat & poultry", "Pork, fresh, loin, center loin (chops), bone-in, separable lean and fat, raw"),
 ("Pork chop (grilled)", ["pork", "pork loin", "chop"], "Meat & poultry", "Pork, fresh, loin, center loin (chops), bone-in, separable lean and fat, cooked, broiled"),
 ("Bacon (raw)", ["rashers", "streaky", "back bacon", "pork"], "Meat & poultry", "Pork, cured, bacon, unprepared"),
 ("Bacon (cooked)", ["rashers", "streaky", "back bacon", "pork"], "Meat & poultry", "Pork, cured, bacon, pre-sliced, cooked, pan-fried"),
 ("Ham (sliced)", ["pork", "deli ham", "cooked ham"], "Meat & poultry", "Ham, sliced, regular (approximately 11% fat)"),
 ("Sausages, pork (raw)", ["sausage", "bangers", "pork"], "Meat & poultry", "Pork sausage, link/patty, unprepared"),
 ("Sausages, pork (cooked)", ["sausage", "bangers", "pork"], "Meat & poultry", "Pork sausage, link/patty, cooked, pan-fried"),
 ("Lamb mince (raw)", ["lamb", "ground lamb"], "Meat & poultry", "Lamb, ground, raw"),
 ("Lamb mince (cooked)", ["lamb", "ground lamb"], "Meat & poultry", "Lamb, ground, cooked, broiled"),
 ("Lamb leg (raw)", ["lamb"], "Meat & poultry", "Lamb, New Zealand, imported, frozen, leg, whole (shank and sirloin), separable lean and fat, trimmed to 1/8\" fat, raw"),
 ("Lamb leg (roast)", ["lamb", "roast lamb"], "Meat & poultry", "Lamb, New Zealand, imported, frozen, leg, whole (shank and sirloin), separable lean and fat, trimmed to 1/8\" fat, cooked, roasted"),
]

# fish and seafood, dairy, bread/rice/pasta, pulses, nuts, oils, sweet things, drinks, cooked vegetables
FOODS += [
 ("Salmon (raw)", ["fish", "salmon fillet"], "Fish & seafood", "Fish, salmon, Atlantic, farmed, raw"),
 ("Salmon (baked)", ["fish", "salmon fillet", "grilled salmon"], "Fish & seafood", "Fish, salmon, Atlantic, farmed, cooked, dry heat"),
 ("Smoked salmon", ["fish", "lox"], "Fish & seafood", "Fish, salmon, chinook, smoked"),
 ("Cod (raw)", ["fish", "cod fillet"], "Fish & seafood", "Fish, cod, Atlantic, raw"),
 ("Cod (cooked)", ["fish", "cod fillet"], "Fish & seafood", "Fish, cod, Atlantic, cooked, dry heat"),
 ("Haddock (raw)", ["fish"], "Fish & seafood", "Fish, haddock, raw"),
 ("Haddock (cooked)", ["fish"], "Fish & seafood", "Fish, haddock, cooked, dry heat"),
 ("Smoked haddock", ["fish", "finnan haddie"], "Fish & seafood", "Fish, haddock, smoked"),
 ("Mackerel (raw)", ["fish"], "Fish & seafood", "Fish, mackerel, Atlantic, raw"),
 ("Mackerel (cooked)", ["fish"], "Fish & seafood", "Fish, mackerel, Atlantic, cooked, dry heat"),
 ("Herring (raw)", ["fish"], "Fish & seafood", "Fish, herring, Atlantic, raw"),
 ("Sardines (canned in oil)", ["fish", "sardine"], "Fish & seafood", "Fish, sardine, Atlantic, canned in oil, drained solids with bone"),
 ("Tuna (canned in water)", ["fish", "tinned tuna"], "Fish & seafood", "Fish, tuna, light, canned in water, drained solids (Includes foods for USDA's Food Distribution Program)"),
 ("Tuna (canned in oil)", ["fish", "tinned tuna"], "Fish & seafood", "Fish, tuna, light, canned in oil, drained solids"),
 ("Tuna steak (raw)", ["fish", "yellowfin"], "Fish & seafood", "Fish, tuna, fresh, yellowfin, raw"),
 ("Trout (raw)", ["fish", "rainbow trout"], "Fish & seafood", "Fish, trout, rainbow, farmed, raw"),
 ("Trout (cooked)", ["fish", "rainbow trout"], "Fish & seafood", "Fish, trout, rainbow, farmed, cooked, dry heat"),
 ("Sea bass (raw)", ["fish"], "Fish & seafood", "Fish, sea bass, mixed species, raw"),
 ("Sea bass (cooked)", ["fish"], "Fish & seafood", "Fish, sea bass, mixed species, cooked, dry heat"),
 ("Plaice / sole (raw)", ["fish", "flounder", "sole", "plaice"], "Fish & seafood", "Fish, flatfish (flounder and sole species), raw"),
 ("Plaice / sole (cooked)", ["fish", "flounder", "sole", "plaice"], "Fish & seafood", "Fish, flatfish (flounder and sole species), cooked, dry heat"),
 ("Pollock / coley (raw)", ["fish", "coley", "pollack"], "Fish & seafood", "Fish, pollock, Atlantic, raw"),
 ("Pollock / coley (cooked)", ["fish", "coley", "pollack"], "Fish & seafood", "Fish, pollock, Atlantic, cooked, dry heat"),
 ("Tilapia (raw)", ["fish"], "Fish & seafood", "Fish, tilapia, raw"),
 ("Tilapia (cooked)", ["fish"], "Fish & seafood", "Fish, tilapia, cooked, dry heat"),
 ("Prawns (raw)", ["shrimp", "seafood"], "Fish & seafood", "Crustaceans, shrimp, mixed species, raw (may contain additives to retain moisture)"),
 ("Prawns (cooked)", ["shrimp", "seafood"], "Fish & seafood", "Crustaceans, shrimp, mixed species, cooked, moist heat (may contain additives to retain moisture)"),
 ("Mussels (cooked)", ["seafood"], "Fish & seafood", "Mollusks, mussel, blue, cooked, moist heat"),
 ("Scallops (raw)", ["seafood"], "Fish & seafood", "Mollusks, scallop, mixed species, raw"),
 ("Crab (cooked)", ["seafood"], "Fish & seafood", "Crustaceans, crab, blue, cooked, moist heat"),
 # dairy
 ("Milk (whole)", ["full fat milk", "full-fat milk", "milk"], "Dairy", "Milk, whole, 3.25% milkfat, with added vitamin D"),
 ("Milk (semi-skimmed)", ["semi skimmed", "2% milk", "milk", "reduced fat milk"], "Dairy", "Milk, reduced fat, fluid, 2% milkfat, with added vitamin A and vitamin D"),
 ("Milk (skimmed)", ["skim milk", "fat free milk", "milk"], "Dairy", "Milk, nonfat, fluid, with added vitamin A and vitamin D (fat free or skim)"),
 ("Buttermilk", ["milk"], "Dairy", "Milk, buttermilk, fluid, cultured, lowfat"),
 ("Yoghurt (plain, whole milk)", ["yogurt", "natural yoghurt"], "Dairy", "Yogurt, plain, whole milk"),
 ("Yoghurt (plain, low fat)", ["yogurt", "natural yoghurt"], "Dairy", "Yogurt, plain, low fat"),
 ("Greek yoghurt (whole milk)", ["yogurt", "greek yogurt"], "Dairy", "Yogurt, Greek, plain, whole milk"),
 ("Greek yoghurt (0% fat)", ["yogurt", "greek yogurt", "fat free"], "Dairy", "Yogurt, Greek, plain, nonfat (Includes foods for USDA's Food Distribution Program)"),
 ("Cheddar cheese", ["cheese"], "Dairy", "Cheese, cheddar (Includes foods for USDA's Food Distribution Program)"),
 ("Mozzarella", ["cheese"], "Dairy", "Cheese, mozzarella, whole milk"),
 ("Feta", ["cheese"], "Dairy", "Cheese, feta"),
 ("Parmesan", ["cheese"], "Dairy", "Cheese, parmesan, hard"),
 ("Cottage cheese", ["cheese"], "Dairy", "Cheese, cottage, creamed, large or small curd"),
 ("Cream cheese", ["cheese", "philadelphia"], "Dairy", "Cheese, cream"),
 ("Brie", ["cheese"], "Dairy", "Cheese, brie"),
 ("Camembert", ["cheese"], "Dairy", "Cheese, camembert"),
 ("Edam", ["cheese"], "Dairy", "Cheese, edam"),
 ("Gouda", ["cheese"], "Dairy", "Cheese, gouda"),
 ("Blue cheese", ["cheese", "stilton", "roquefort"], "Dairy", "Cheese, blue"),
 ("Goat's cheese (soft)", ["cheese", "chevre"], "Dairy", "Cheese, goat, soft type"),
 ("Butter (salted)", ["butter", "spread"], "Fats & oils", "Butter, salted"),
 ("Single cream", ["cream", "coffee cream"], "Dairy", "Cream, fluid, light (coffee cream or table cream)"),
 ("Double cream", ["cream", "whipping cream"], "Dairy", "Cream, fluid, heavy whipping"),
 ("Soured cream", ["cream", "sour cream"], "Dairy", "Cream, sour, cultured"),
 ("Ice cream (vanilla)", ["dessert"], "Sweet & spreads", "Ice creams, vanilla"),
 ("Soya milk (unsweetened)", ["soy milk", "plant milk", "milk"], "Dairy", "Soymilk (all flavors), unsweetened, with added calcium, vitamins A and D"),
 ("Almond milk (unsweetened)", ["plant milk", "milk"], "Dairy", "Beverages, almond milk, unsweetened, shelf stable"),
 ("Rice milk (unsweetened)", ["plant milk", "milk"], "Dairy", "Beverages, rice milk, unsweetened"),
 ("Tofu (firm)", ["soya", "soy", "plant protein"], "Beans & pulses", "Tofu, raw, firm, prepared with calcium sulfate"),
 # bread, rice, pasta and grains
 ("Bread (white)", ["sliced bread", "toast"], "Bread, rice & pasta", "Bread, white, commercially prepared (includes soft bread crumbs)"),
 ("Bread (wholemeal)", ["whole wheat", "brown bread", "sliced bread"], "Bread, rice & pasta", "Bread, whole-wheat, commercially prepared"),
 ("Toast (white)", ["bread"], "Bread, rice & pasta", "Bread, white, commercially prepared, toasted"),
 ("Toast (wholemeal)", ["bread", "whole wheat"], "Bread, rice & pasta", "Bread, whole-wheat, commercially prepared, toasted"),
 ("Bread (rye)", ["sourdough rye"], "Bread, rice & pasta", "Bread, rye"),
 ("Bread (multigrain)", ["seeded bread", "granary"], "Bread, rice & pasta", "Bread, multi-grain (includes whole-grain)"),
 ("Pitta bread (white)", ["pita"], "Bread, rice & pasta", "Bread, pita, white, enriched"),
 ("Naan bread", ["bread"], "Bread, rice & pasta", "Bread, naan, plain, commercially prepared, refrigerated"),
 ("Rice (white, cooked)", ["boiled rice"], "Bread, rice & pasta", "Rice, white, long-grain, regular, enriched, cooked"),
 ("Rice (brown, cooked)", ["boiled rice", "wholegrain rice"], "Bread, rice & pasta", "Rice, brown, long-grain, cooked (Includes foods for USDA's Food Distribution Program)"),
 ("Rice (white, raw)", ["dry rice", "uncooked rice"], "Bread, rice & pasta", "Rice, white, long-grain, regular, raw, enriched"),
 ("Rice (brown, raw)", ["dry rice", "uncooked rice"], "Bread, rice & pasta", "Rice, brown, long-grain, raw (Includes foods for USDA's Food Distribution Program)"),
 ("Pasta (cooked)", ["spaghetti", "penne", "macaroni", "boiled pasta"], "Bread, rice & pasta", "Pasta, cooked, enriched, without added salt"),
 ("Pasta (dry)", ["spaghetti", "penne", "macaroni", "uncooked pasta"], "Bread, rice & pasta", "Pasta, dry, enriched"),
 ("Pasta (wholewheat, cooked)", ["spaghetti", "whole wheat pasta"], "Bread, rice & pasta", "Pasta, whole-wheat, cooked (Includes foods for USDA's Food Distribution Program)"),
 ("Egg noodles (cooked)", ["noodles"], "Bread, rice & pasta", "Noodles, egg, enriched, cooked"),
 ("Soba noodles (cooked)", ["noodles", "buckwheat"], "Bread, rice & pasta", "Noodles, japanese, soba, cooked"),
 ("Couscous (cooked)", [], "Bread, rice & pasta", "Couscous, cooked"),
 ("Quinoa (cooked)", [], "Bread, rice & pasta", "Quinoa, cooked"),
 ("Bulgur wheat (cooked)", ["bulgar"], "Bread, rice & pasta", "Bulgur, cooked"),
 ("Porridge oats (dry)", ["oats", "rolled oats", "oatmeal"], "Bread, rice & pasta", "Cereals, oats, regular and quick, not fortified, dry"),
 ("Porridge (made with water)", ["oats", "oatmeal"], "Bread, rice & pasta", "Cereals, oats, regular and quick, unenriched, cooked with water (includes boiling and microwaving), without salt"),
 ("Plain flour", ["flour", "wheat flour"], "Bread, rice & pasta", "Wheat flour, white, all-purpose, enriched, unbleached"),
 # potatoes and other cooked vegetables
 ("Potatoes (boiled)", ["potato", "boiled potatoes"], "Vegetables", "Potatoes, boiled, cooked without skin, flesh, without salt"),
 ("Potatoes (baked, with skin)", ["potato", "jacket potato", "baked potato"], "Vegetables", "Potatoes, baked, flesh and skin, without salt"),
 ("Mashed potato", ["potato", "mash"], "Vegetables", "Potatoes, mashed, home-prepared, whole milk and margarine added"),
 ("Chips (oven-baked)", ["potato", "fries", "french fries"], "Vegetables", "Potatoes, french fried, all types, salt added in processing, frozen, home-prepared, oven heated"),
 ("Sweet potato (baked)", ["potato"], "Vegetables", "Sweet potato, cooked, baked in skin, flesh, without salt"),
 ("Broccoli (boiled)", [], "Vegetables", "Broccoli, cooked, boiled, drained, without salt"),
 ("Carrots (boiled)", ["carrot"], "Vegetables", "Carrots, cooked, boiled, drained, without salt"),
 ("Peas (boiled)", ["garden peas"], "Vegetables", "Peas, green, cooked, boiled, drained, without salt"),
 ("Cauliflower (boiled)", [], "Vegetables", "Cauliflower, cooked, boiled, drained, without salt"),
 ("Spinach (boiled)", [], "Vegetables", "Spinach, cooked, boiled, drained, without salt"),
 ("Sweetcorn (boiled)", ["corn"], "Vegetables", "Corn, sweet, yellow, cooked, boiled, drained, without salt"),
 ("Green beans (boiled)", ["french beans", "runner beans"], "Vegetables", "Beans, snap, green, cooked, boiled, drained, without salt"),
 ("Cabbage (boiled)", [], "Vegetables", "Cabbage, cooked, boiled, drained, without salt"),
 ("Brussels sprouts (boiled)", ["sprouts"], "Vegetables", "Brussels sprouts, cooked, boiled, drained, without salt"),
 ("Parsnips (boiled)", ["parsnip"], "Vegetables", "Parsnips, cooked, boiled, drained, without salt"),
 ("Beetroot (boiled)", ["beets"], "Vegetables", "Beets, cooked, boiled, drained"),
 ("Butternut squash (baked)", ["squash"], "Vegetables", "Squash, winter, butternut, cooked, baked, without salt"),
 # beans and pulses
 ("Baked beans", ["beans", "heinz", "tinned beans"], "Beans & pulses", "Beans, baked, canned, plain or vegetarian"),
 ("Kidney beans (cooked)", ["beans", "red kidney beans"], "Beans & pulses", "Beans, kidney, all types, mature seeds, cooked, boiled, without salt"),
 ("Black beans (cooked)", ["beans"], "Beans & pulses", "Beans, black, mature seeds, cooked, boiled, without salt"),
 ("Cannellini beans (cooked)", ["beans", "white beans"], "Beans & pulses", "Beans, white, mature seeds, cooked, boiled, without salt"),
 ("Chickpeas (cooked)", ["garbanzo"], "Beans & pulses", "Chickpeas (garbanzo beans, bengal gram), mature seeds, cooked, boiled, without salt"),
 ("Chickpeas (canned, drained)", ["garbanzo", "tinned chickpeas"], "Beans & pulses", "Chickpeas (garbanzo beans, bengal gram), mature seeds, canned, drained solids"),
 ("Lentils (cooked)", [], "Beans & pulses", "Lentils, mature seeds, cooked, boiled, without salt"),
 ("Lentils (dry)", ["uncooked lentils"], "Beans & pulses", "Lentils, raw"),
 ("Hummus", ["houmous", "dip"], "Beans & pulses", "Hummus, commercial"),
 # nuts and seeds
 ("Almonds", ["nuts"], "Nuts & seeds", "Nuts, almonds"),
 ("Walnuts", ["nuts"], "Nuts & seeds", "Nuts, walnuts, english"),
 ("Cashew nuts", ["nuts"], "Nuts & seeds", "Nuts, cashew nuts, raw"),
 ("Peanuts", ["nuts", "groundnuts"], "Nuts & seeds", "Peanuts, all types, raw"),
 ("Hazelnuts", ["nuts", "filberts"], "Nuts & seeds", "Nuts, hazelnuts or filberts"),
 ("Brazil nuts", ["nuts"], "Nuts & seeds", "Nuts, brazilnuts, dried, unblanched"),
 ("Pecans", ["nuts"], "Nuts & seeds", "Nuts, pecans"),
 ("Pistachios", ["nuts"], "Nuts & seeds", "Nuts, pistachio nuts, raw"),
 ("Peanut butter (smooth)", ["nuts", "spread"], "Nuts & seeds", "Peanut butter, smooth style, with salt (Includes foods for USDA's Food Distribution Program)"),
 ("Chia seeds", ["seeds"], "Nuts & seeds", "Seeds, chia seeds, dried"),
 ("Flaxseed", ["seeds", "linseed"], "Nuts & seeds", "Seeds, flaxseed"),
 ("Pumpkin seeds", ["seeds"], "Nuts & seeds", "Seeds, pumpkin and squash seed kernels, dried"),
 ("Sunflower seeds", ["seeds"], "Nuts & seeds", "Seeds, sunflower seed kernels, dried"),
 # oils, sweet things and drinks
 ("Olive oil", ["oil", "cooking oil"], "Fats & oils", "Oil, olive, salad or cooking"),
 ("Rapeseed oil", ["canola", "oil", "cooking oil"], "Fats & oils", "Oil, canola"),
 ("Sunflower oil", ["oil", "cooking oil"], "Fats & oils", "Oil, sunflower, linoleic (less than 60%)"),
 ("Coconut oil", ["oil"], "Fats & oils", "Oil, coconut"),
 ("Sugar", ["white sugar", "granulated sugar"], "Sweet & spreads", "Sugars, granulated"),
 ("Honey", [], "Sweet & spreads", "Honey"),
 ("Jam", ["preserves", "marmalade", "spread"], "Sweet & spreads", "Jams and preserves"),
 ("Dark chocolate", ["chocolate"], "Sweet & spreads", "Candies, chocolate, dark, NFS (45-59% cacao solids 90%; 60-69% cacao solids 5%; 70-85% cacao solids 5%)"),
 ("Milk chocolate", ["chocolate"], "Sweet & spreads", "Candies, milk chocolate"),
 ("Orange juice", ["juice"], "Drinks", "Orange juice, raw (Includes foods for USDA's Food Distribution Program)"),
 ("Apple juice", ["juice"], "Drinks", "Apple juice, canned or bottled, unsweetened, without added ascorbic acid"),
 ("Cola", ["fizzy drink", "coke", "soft drink"], "Drinks", "Beverages, carbonated, cola, regular"),
 ("Coffee (black)", ["tea and coffee"], "Drinks", "Beverages, coffee, brewed, prepared with tap water"),
 ("Tea (black, no milk)", ["tea and coffee"], "Drinks", "Beverages, tea, black, brewed, prepared with tap water"),
]

foods = {r['description']: r['fdc_id'] for r in csv.DictReader(open(data + '/food.csv', newline='', encoding='utf-8'))}
IDS = {'1008': 'calories', '1003': 'protein', '1005': 'carbs', '1004': 'fat', '1079': 'fiber'}  # energy (kcal), protein, carbs, fat, fibre per 100 g
want = {}
missing = []
for name, _, _, desc in FOODS:
    if desc not in foods:
        missing.append((name, desc))
    else:
        want[foods[desc]] = name
if missing:
    print('NOT FOUND:', missing); sys.exit(1)
vals = {}
for r in csv.DictReader(open(data + '/food_nutrient.csv', newline='', encoding='utf-8')):
    if r['fdc_id'] in want and r['nutrient_id'] in IDS:
        vals.setdefault(r['fdc_id'], {})[IDS[r['nutrient_id']]] = float(r['amount'])
# Typical portions (one medium banana, one large egg, one slice...) from USDA's own records,
# so a meal can be logged as "2 eggs" rather than grams. Only "1 x" portions that are a piece,
# a slice or a measured cup are kept; sizes first, then slices, then cups.
import re
PRIORITY = [(re.compile(r'^(extra small|small|medium|large|extra large|jumbo)\b'), 1), (re.compile(r'^(slice|piece|link|patty|pat|rasher|fillet|breast|wedge|cherry|plum|italian|bar|stick|can|pita|naan|bagel)\b'), 2), (re.compile(r'^(tbsp|tsp)\b'), 3), (re.compile(r'^cup\b'), 4)]
SIZE_ORDER = {'medium': 0, 'large': 1, 'small': 2, 'extra large': 3, 'extra small': 4, 'jumbo': 5}
SPOONS_OK = {'Fats & oils', 'Sweet & spreads', 'Nuts & seeds'}      # a tablespoon of oil or honey, not of milk
NO_CUPS = {'Nuts & seeds', 'Fats & oils', 'Sweet & spreads'}          # nobody measures nuts or oil by the cup
ODD = re.compile(r'crust|packed|elbows|farfalle|lasagne|melted|ground|slivered|not eaten|thin\b|noodles|spaghetti|penne|rotini|shells|crumbs|cubes')
cat_of = {foods[desc]: cat for _, _, cat, desc in FOODS}
by_id = {}
for r in csv.DictReader(open(data + '/food_portion.csv', newline='', encoding='utf-8')):
    if r['fdc_id'] not in want:
        continue
    try:
        amount, grams = float(r['amount']), float(r['gram_weight'])
    except ValueError:
        continue
    if amount <= 0 or grams <= 0:
        continue
    mod = re.sub(r'\s*\(.*$', '', r['modifier'].strip()).strip()
    if ODD.search(mod):
        continue
    pr = next((rank for rx, rank in PRIORITY if rx.match(mod)), None)
    cat = cat_of[r['fdc_id']]
    if pr is None or (pr == 3 and cat not in SPOONS_OK) or (pr == 4 and cat in NO_CUPS):
        continue
    if amount != 1 and pr not in (3, 4):   # "10 rings" or "3 oz" aren't one-of-something; spoons and cups scale
        continue
    size = next((k for k in SIZE_ORDER if mod.startswith(k + ' ') or mod == k), None)
    by_id.setdefault(r['fdc_id'], {}).setdefault(mod, (pr, SIZE_ORDER.get(size, 9), round(grams / amount, 1)))
def portions_for(fid):
    items = sorted(by_id.get(fid, {}).items(), key=lambda kv: (kv[1][0], kv[1][1], kv[0]))
    return [{'label': '1 ' + label, 'grams': g} for label, (_, _, g) in items[:4]]

result = []
for name, aliases, cat, desc in FOODS:
    fid = foods[desc]; v = vals.get(fid, {})
    if 'calories' not in v:
        print('NO ENERGY for', name, desc); sys.exit(1)
    result.append({
        'name': name, 'aliases': aliases, 'category': cat,
        'calories': round(v['calories']), 'protein': round(v.get('protein', 0), 1), 'carbs': round(v.get('carbs', 0), 1),
        'fat': round(v.get('fat', 0), 1), 'fiber': round(v.get('fiber', 0), 1),
        'portions': portions_for(fid),
        'fdcId': int(fid), 'usda': desc.replace(" (Includes foods for USDA's Food Distribution Program)", ''),
    })
# Preserve the separately sourced FNDDS coffee records on a full rebuild.
from pathlib import Path
import runpy
result.extend(runpy.run_path(str(Path(__file__).with_name('build_coffees.py')))['coffee_foods']())
json.dump(result, open(out, 'w', encoding='utf-8'), indent=1, ensure_ascii=False)
print(f'{len(result)} foods written')
for r in [x for x in result if x['name'] in ('Eggs (boiled)', 'Chicken breast (cooked)', 'Bananas', 'Potatoes (white)', 'Bacon (cooked)', 'Sausages, pork (cooked)', 'Beef mince 10% fat (cooked)', 'Tomatoes', 'Apples')]:
    print(f"  {r['name']:30} {r['calories']:>4} kcal  P{r['protein']:>5}  C{r['carbs']:>5}  F{r['fat']:>5}  fibre{r['fiber']:>4}  portions: {[(q['label'], q['grams']) for q in r['portions']]}")
print('without portions:', [x['name'] for x in result if not x['portions']])
