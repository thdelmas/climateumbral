// Per-city landing pages — the receiving surface for distribution.
// A person searching "refugio climático Madrid" or a journalist
// linking "cool places in Wien" must land on a server-rendered page
// in that city's language that answers immediately and hands off to
// the finder. The SPA can't do this (one English page, no URLs per
// city); these routes can. Static HTML, no JS required to read it.
package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type cityPage struct {
	Slug string
	Name string
	Lang string
	Lon  float64
	Lat  float64
}

// The sweep set: every city verified to show cool places on the
// live map (2026-07-20), adapter cities first.
var cityPages = []cityPage{
	{Slug: "barcelona", Name: "Barcelona", Lang: "ca",
		Lon: 2.17, Lat: 41.387},
	{Slug: "paris", Name: "Paris", Lang: "fr",
		Lon: 2.352, Lat: 48.857},
	{Slug: "wien", Name: "Wien", Lang: "de",
		Lon: 16.372, Lat: 48.208},
	{Slug: "lyon", Name: "Lyon", Lang: "fr",
		Lon: 4.835, Lat: 45.758},
	{Slug: "madrid", Name: "Madrid", Lang: "es",
		Lon: -3.703, Lat: 40.417},
	{Slug: "sevilla", Name: "Sevilla", Lang: "es",
		Lon: -5.994, Lat: 37.389},
	{Slug: "roma", Name: "Roma", Lang: "it",
		Lon: 12.496, Lat: 41.903},
	{Slug: "milano", Name: "Milano", Lang: "it",
		Lon: 9.19, Lat: 45.464},
	{Slug: "lisboa", Name: "Lisboa", Lang: "pt",
		Lon: -9.139, Lat: 38.722},
	{Slug: "athens", Name: "Αθήνα", Lang: "el",
		Lon: 23.728, Lat: 37.984},
	{Slug: "warszawa", Name: "Warszawa", Lang: "pl",
		Lon: 21.012, Lat: 52.23},
	{Slug: "amsterdam", Name: "Amsterdam", Lang: "nl",
		Lon: 4.895, Lat: 52.37},
	{Slug: "brussels", Name: "Bruxelles", Lang: "fr",
		Lon: 4.352, Lat: 50.847},
	{Slug: "munich", Name: "München", Lang: "de",
		Lon: 11.576, Lat: 48.137},
	{Slug: "berlin", Name: "Berlin", Lang: "de",
		Lon: 13.4, Lat: 52.52},
	{Slug: "praha", Name: "Praha", Lang: "cs",
		Lon: 14.421, Lat: 50.088},
}

type pageText struct {
	Title      string // %s = city
	H1         string // %s = city
	Intro      string
	CtaFinder  string
	CtaMap     string
	Shelters   string // %d = count
	NoShelters string
	Advice     string
}

var pageTexts = map[string]pageText{
	"en": {
		Title: "Climate shelters and cool places in %s — ClimateUmbral",
		H1:    "Too hot in %s?",
		Intro: "Find an official climate shelter or an air-conditioned " +
			"public place near you — free, no app, no account.",
		CtaFinder: "Find a cool place now",
		CtaMap:    "Open the map",
		Shelters: "%d official climate shelters published by the city are " +
			"on the map.",
		NoShelters: "Your city has not published a climate-shelter list yet — " +
			"the map still shows air-conditioned public places from " +
			"OpenStreetMap, and heat advice in your language.",
		Advice: "Drink water often. Wet your skin. Avoid the sun from " +
			"12:00 to 17:00. Dizzy or confused? Call 112.",
	},
	"es": {
		Title: "Refugios climáticos y lugares frescos en %s — ClimateUmbral",
		H1:    "¿Demasiado calor en %s?",
		Intro: "Encuentra un refugio climático oficial o un lugar público " +
			"climatizado cerca de ti — gratis, sin app, sin cuenta.",
		CtaFinder: "Buscar un lugar fresco ahora",
		CtaMap:    "Abrir el mapa",
		Shelters: "%d refugios climáticos oficiales publicados por la ciudad " +
			"están en el mapa.",
		NoShelters: "Tu ciudad aún no publica una lista de refugios " +
			"climáticos — el mapa muestra igualmente lugares " +
			"públicos climatizados de OpenStreetMap y consejos " +
			"contra el calor.",
		Advice: "Bebe agua a menudo. Mójate la piel. Evita el sol de " +
			"12:00 a 17:00. ¿Mareo o confusión? Llama al 112.",
	},
	"ca": {
		Title: "Refugis climàtics i llocs frescos a %s — ClimateUmbral",
		H1:    "Massa calor a %s?",
		Intro: "Troba un refugi climàtic oficial o un lloc públic " +
			"climatitzat a prop teu — gratuït, sense app, sense compte.",
		CtaFinder: "Troba un lloc fresc ara",
		CtaMap:    "Obre el mapa",
		Shelters: "%d refugis climàtics oficials publicats per la ciutat " +
			"són al mapa.",
		NoShelters: "La teva ciutat encara no publica una llista de refugis " +
			"climàtics — el mapa mostra igualment llocs públics " +
			"climatitzats d'OpenStreetMap i consells contra la calor.",
		Advice: "Beu aigua sovint. Mulla't la pell. Evita el sol de 12:00 " +
			"a 17:00. Mareig o confusió? Truca al 112.",
	},
	"fr": {
		Title: "Refuges climatiques et lieux frais à %s — ClimateUmbral",
		H1:    "Trop chaud à %s ?",
		Intro: "Trouvez un refuge climatique officiel ou un lieu public " +
			"climatisé près de chez vous — gratuit, sans " +
			"application, sans compte.",
		CtaFinder: "Trouver un lieu frais maintenant",
		CtaMap:    "Ouvrir la carte",
		Shelters: "%d refuges climatiques officiels publiés par la ville " +
			"sont sur la carte.",
		NoShelters: "Votre ville ne publie pas encore de liste de refuges — " +
			"la carte montre quand même les lieux publics " +
			"climatisés d'OpenStreetMap, et des conseils contre " +
			"la chaleur.",
		Advice: "Buvez de l'eau souvent. Mouillez votre peau. Évitez le " +
			"soleil de 12 h à 17 h. Vertiges, confusion ? " +
			"Appelez le 112.",
	},
	"de": {
		Title: "Kühle Orte und Klima-Schutzräume in %s — ClimateUmbral",
		H1:    "Zu heiß in %s?",
		Intro: "Finden Sie einen offiziellen kühlen Ort oder einen " +
			"klimatisierten öffentlichen Raum in Ihrer Nähe — " +
			"kostenlos, ohne App, ohne Konto.",
		CtaFinder: "Jetzt kühlen Ort finden",
		CtaMap:    "Karte öffnen",
		Shelters:  "%d offizielle kühle Orte der Stadt sind auf der Karte.",
		NoShelters: "Ihre Stadt veröffentlicht noch keine Liste kühler Orte — " +
			"die Karte zeigt trotzdem klimatisierte öffentliche " +
			"Orte aus OpenStreetMap und Hitzetipps.",
		Advice: "Trinken Sie oft Wasser. Machen Sie die Haut nass. Meiden " +
			"Sie die Sonne von 12 bis 17 Uhr. Schwindel oder " +
			"Verwirrung? Rufen Sie 112 an.",
	},
	"it": {
		Title: "Rifugi climatici e luoghi freschi a %s — ClimateUmbral",
		H1:    "Troppo caldo a %s?",
		Intro: "Trova un rifugio climatico ufficiale o un luogo pubblico " +
			"climatizzato vicino a te — gratis, senza app, senza " +
			"account.",
		CtaFinder: "Trova subito un luogo fresco",
		CtaMap:    "Apri la mappa",
		Shelters: "%d rifugi climatici ufficiali pubblicati dalla città " +
			"sono sulla mappa.",
		NoShelters: "La tua città non pubblica ancora una lista di rifugi " +
			"climatici — la mappa mostra comunque i luoghi " +
			"pubblici climatizzati da OpenStreetMap e consigli " +
			"contro il caldo.",
		Advice: "Bevi acqua spesso. Bagnati la pelle. Evita il sole dalle " +
			"12:00 alle 17:00. Vertigini o confusione? Chiama il 112.",
	},
	"pt": {
		Title: "Refúgios climáticos e lugares frescos em %s — ClimateUmbral",
		H1:    "Calor demais em %s?",
		Intro: "Encontre um refúgio climático oficial ou um lugar " +
			"público climatizado perto de si — grátis, sem " +
			"aplicação, sem conta.",
		CtaFinder: "Encontrar um lugar fresco agora",
		CtaMap:    "Abrir o mapa",
		Shelters: "%d refúgios climáticos oficiais publicados pela cidade " +
			"estão no mapa.",
		NoShelters: "A sua cidade ainda não publica uma lista de refúgios " +
			"climáticos — o mapa mostra na mesma lugares públicos " +
			"climatizados do OpenStreetMap e conselhos contra o calor.",
		Advice: "Beba água muitas vezes. Molhe a pele. Evite o sol das " +
			"12:00 às 17:00. Tonturas ou confusão? Ligue 112.",
	},
	"nl": {
		Title: "Koele plekken en klimaatschuilplaatsen in %s — ClimateUmbral",
		H1:    "Te heet in %s?",
		Intro: "Vind een officiële koele plek of een openbaar gebouw met " +
			"airco bij jou in de buurt — gratis, zonder app, " +
			"zonder account.",
		CtaFinder: "Vind nu een koele plek",
		CtaMap:    "Open de kaart",
		Shelters:  "%d officiële koele plekken van de stad staan op de kaart.",
		NoShelters: "Je stad publiceert nog geen lijst met koele plekken — de " +
			"kaart toont wel openbare plekken met airco uit " +
			"OpenStreetMap en tips tegen de hitte.",
		Advice: "Drink vaak water. Maak je huid nat. Vermijd de zon van " +
			"12:00 tot 17:00. Duizelig of verward? Bel 112.",
	},
	"pl": {
		Title: "Miejsca chłodu w %s — ClimateUmbral",
		H1:    "Za gorąco w %s?",
		Intro: "Znajdź oficjalne miejsce chłodu lub klimatyzowane " +
			"miejsce publiczne w pobliżu — za darmo, bez " +
			"aplikacji, bez konta.",
		CtaFinder: "Znajdź chłodne miejsce teraz",
		CtaMap:    "Otwórz mapę",
		Shelters: "%d oficjalnych miejsc chłodu opublikowanych przez miasto " +
			"jest na mapie.",
		NoShelters: "Twoje miasto nie opublikowało jeszcze listy miejsc " +
			"chłodu — mapa i tak pokazuje klimatyzowane miejsca " +
			"publiczne z OpenStreetMap i porady na upał.",
		Advice: "Pij często wodę. Zwilżaj skórę. Unikaj słońca od 12:00 " +
			"do 17:00. Zawroty głowy, dezorientacja? Dzwoń pod 112.",
	},
	"cs": {
		Title: "Chladná místa v %s — ClimateUmbral",
		H1:    "Příliš horko v %s?",
		Intro: "Najděte oficiální chladné místo nebo klimatizované " +
			"veřejné místo ve svém okolí — zdarma, bez aplikace, " +
			"bez účtu.",
		CtaFinder: "Najít chladné místo teď",
		CtaMap:    "Otevřít mapu",
		Shelters: "%d oficiálních chladných míst zveřejněných městem je " +
			"na mapě.",
		NoShelters: "Vaše město zatím seznam chladných míst nezveřejnilo — " +
			"mapa přesto ukazuje klimatizovaná veřejná místa z " +
			"OpenStreetMap a rady proti horku.",
		Advice: "Pijte často vodu. Namočte si pokožku. Vyhýbejte se " +
			"slunci od 12:00 do 17:00. Závratě nebo zmatenost? " +
			"Volejte 112.",
	},
	"el": {
		Title: "Κλιματικά καταφύγια και δροσερά μέρη — %s — ClimateUmbral",
		H1:    "Πολλή ζέστη στην %s;",
		Intro: "Βρείτε ένα επίσημο κλιματικό καταφύγιο ή έναν " +
			"κλιματιζόμενο δημόσιο χώρο κοντά σας — δωρεάν, χωρίς " +
			"εφαρμογή, χωρίς λογαριασμό.",
		CtaFinder: "Βρείτε δροσερό μέρος τώρα",
		CtaMap:    "Ανοίξτε τον χάρτη",
		Shelters:  "%d επίσημα κλιματικά καταφύγια της πόλης είναι στον χάρτη.",
		NoShelters: "Η πόλη σας δεν έχει δημοσιεύσει ακόμη λίστα καταφυγίων — " +
			"ο χάρτης δείχνει κλιματιζόμενους δημόσιους χώρους από " +
			"το OpenStreetMap και συμβουλές για τον καύσωνα.",
		Advice: "Πίνετε νερό συχνά. Βρέξτε το δέρμα σας. Αποφύγετε τον " +
			"ήλιο 12:00–17:00. Ζάλη ή σύγχυση; Καλέστε το 112.",
	},
}

var cityTmpl = template.Must(template.New("city").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Intro}}">
<link rel="canonical" href="https://climateumbral.eu/{{.Slug}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Intro}}">
<meta property="og:image" content="https://climateumbral.eu/og.png">
<meta property="og:url" content="https://climateumbral.eu/{{.Slug}}">
<meta property="og:type" content="website">
<style>
:root{--bg:#f4f6f2;--ink:#232a26;--ink2:#5a655e;--card:#fbfcfa;
--line:#d8ded8;--cool:#3673a3;--accent:#2e6b3e}
@media (prefers-color-scheme: dark){:root{--bg:#171b18;--ink:#e6ebe6;
--ink2:#a7b1a9;--card:#1e231f;--line:#2e352f;--cool:#7db3dd;
--accent:#7fb98a}}
*{margin:0;padding:0;box-sizing:border-box}
body{background:var(--bg);color:var(--ink);line-height:1.5;
font-family:ui-sans-serif,system-ui,-apple-system,'Segoe UI',sans-serif;
padding:24px 16px calc(48px + env(safe-area-inset-bottom,0px))}
main{max-width:560px;margin:0 auto}
h1{font-size:32px;margin:16px 0 10px;letter-spacing:-.01em}
p{margin:10px 0;font-size:18px}
.muted{color:var(--ink2);font-size:16px}
a.btn{display:block;text-align:center;padding:16px;margin:12px 0;
border-radius:14px;font-size:20px;font-weight:700;
text-decoration:none;color:#fff;min-height:56px}
.finder{background:var(--cool)}
.map{background:var(--accent)}
@media (prefers-color-scheme: dark){a.btn{color:#10130f}}
.advice{margin-top:20px;padding:14px;border:1px solid var(--line);
border-radius:12px;background:var(--card);font-size:17px}
footer{margin-top:28px;font-size:13.5px;color:var(--ink2)}
footer a{color:var(--ink2)}
</style>
</head>
<body>
<main>
<h1>{{.H1}}</h1>
<p>{{.Intro}}</p>
<a class="btn finder" href="/#cool">{{.CtaFinder}}</a>
<a class="btn map" href="/{{.MapHash}}">{{.CtaMap}}</a>
<p>{{.SheltersLine}}</p>
<div class="advice">{{.Advice}}</div>
<footer>
<p>ClimateUmbral — open data, open source.
<a href="/press.html">Press</a> ·
<a href="/poster.html">Poster</a> ·
<a href="https://github.com/thdelmas/climateumbral">GitHub</a></p>
</footer>
</main>
</body>
</html>
`))

// cityShelterCount: official refuges within walking-ish reach of the
// city center (25 km covers a metro area without borrowing the next
// city's network).
func (s *server) cityShelterCount(c cityPage) int {
	n := 0
	for _, src := range refugeSources {
		list, err := s.refuges.get(src)
		if err != nil {
			continue
		}
		for _, r := range list {
			dx := (r.Lon - c.Lon) * 0.71 // ~cos(45°), close enough
			dy := r.Lat - c.Lat
			if (dx*dx+dy*dy)*111.32*111.32 < 25*25 {
				n++
			}
		}
	}
	return n
}

func (s *server) cityPageHandler(c cityPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.counters.bump("page_" + c.Slug) // server-side; whitelist
		// guards only client input
		t, ok := pageTexts[c.Lang]
		if !ok {
			t = pageTexts["en"]
		}
		count := s.cityShelterCount(c)
		line := t.NoShelters
		if count > 0 {
			line = fmt.Sprintf(t.Shelters, count)
		}
		// The map CTA must arrive IN the city, not on the
		// continent: link the map's own #pe,pn permalink.
		E, N := toLAEA(c.Lon, c.Lat)
		mapHash := fmt.Sprintf("#%d,%d",
			int(E/10), int(N/10))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = cityTmpl.Execute(w, map[string]any{
			"Lang":         c.Lang,
			"MapHash":      mapHash,
			"Slug":         c.Slug,
			"Title":        fmt.Sprintf(t.Title, c.Name),
			"H1":           fmt.Sprintf(t.H1, c.Name),
			"Intro":        t.Intro,
			"CtaFinder":    t.CtaFinder,
			"CtaMap":       t.CtaMap,
			"SheltersLine": line,
			"Advice":       t.Advice,
		})
	}
}

func handleSitemap(w http.ResponseWriter, _ *http.Request) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` +
		"\n")
	for _, u := range []string{"", "press.html", "poster.html"} {
		b.WriteString("<url><loc>https://climateumbral.eu/" + u +
			"</loc></url>\n")
	}
	for _, c := range cityPages {
		b.WriteString("<url><loc>https://climateumbral.eu/" +
			c.Slug + "</loc></url>\n")
	}
	b.WriteString("</urlset>\n")
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(b.String()))
}

func handleRobots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(
		"User-agent: *\nAllow: /\n" +
			"Sitemap: https://climateumbral.eu/sitemap.xml\n"))
}
