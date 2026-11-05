module github.com/you/nutrition-app/mobile

go 1.26.0

require github.com/you/nutrition-app v0.0.0

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	golang.org/x/mobile v0.0.0-20261011034528-197770444867 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

// The app lives one directory up; this module only adds the gomobile entry point
// (and, at build time, golang.org/x/mobile/bind) without touching the main go.mod.
replace github.com/you/nutrition-app => ../
