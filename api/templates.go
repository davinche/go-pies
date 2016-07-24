package api

import (
	"html/template"
	"log"
)

const list = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>Go Pies</title>
	<style>
		html { margin: 0; padding: 0 }
		img { max-width: 500px; }
		body {
			margin: 0;
			padding: 0;
			color: #777;
		}

		h1 {
			margin: 20px auto;
			max-width: 960px;
			text-align: center;
		}
		div {
			margin: 20px auto 0;
			padding: 0 20px;
			max-width: 960px;
		}

		div + div {
			padding-top: 20px;
			border-top: 3px solid #ccc;
		}
	</style>
</head>
<body>
	<h1>Pies - Go have a taste heaven</h1>
	{{range $index, $pie := .}}
	<div>
		<p>
			<strong>Name: </strong> <a href="{{.Permalink}}">{{.Name}}</a>
		</p>

		<p>
			<img src="{{.ImageURL}}">
		</p>

		<p>
			<strong>Price:</strong> {{.Price}}
		</p>

		<p>
			<strong>Remaining:</strong> {{.Slices}}
		</p>
	</div>
	{{end}}
</body>
</html>
`

const single = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>{{.Name}}</title>
	<style>
		html { margin: 0; padding: 0 }
		img { max-width: 500px; }
		body {
			margin: 0;
			padding: 0;
			color: #777;
		}

		h1 {
			margin: 20px auto;
			max-width: 960px;
			text-align: center;
		}
		div {
			margin: 20px auto 0;
			padding: 0 20px;
			max-width: 960px;
		}

		div + div {
			padding-top: 20px;
			border-top: 3px solid #ccc;
		}
	</style>
</head>
<body>
	<h1>{{ .Name }}</h1>
	<div>
		<p>
			<strong>Name: </strong> <a href="{{.Permalink}}">{{.Name}}</a>
		</p>

		<p>
			<img src="{{.ImageURL}}">
		</p>

		<p>
			<strong>Price:</strong> {{.Price}}
		</p>

		<p>
			<strong>Remaining:</strong> {{.RemainingSlices}}
		</p>

		{{ if .Purchases }}
		<p><strong>Purchasers</strong></p>
		<ul>
			{{ range .Purchases }}
			<li><strong>{{ .Username}}</strong> - {{ .Slices}} slice{{ if gt .Slices 1}}s{{ end }}</li>
			{{ end }}
		</ul>
		{{ end }}
	</div>
</body>
</html>
`

// PiesList is the template for showing a list of pies
var PiesList *template.Template

// PiesSingle is the template for showing a specific pie
var PiesSingle *template.Template

func init() {
	var err error
	PiesList, err = template.New("PiesList").Parse(list)

	if err != nil {
		log.Fatalf("error: could not parse template: err=%q\n", err)
	}

	PiesSingle, err = template.New("PiesList").Parse(single)

	if err != nil {
		log.Fatalf("error: could not parse template: err=%q\n", err)
	}

}
