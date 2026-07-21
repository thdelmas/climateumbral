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
	{"barcelona", "Barcelona", "ca", 2.17, 41.387},
	{"paris", "Paris", "fr", 2.352, 48.857},
	{"wien", "Wien", "de", 16.372, 48.208},
	{"lyon", "Lyon", "fr", 4.835, 45.758},
	{"madrid", "Madrid", "es", -3.703, 40.417},
	{"sevilla", "Sevilla", "es", -5.994, 37.389},
	{"roma", "Roma", "it", 12.496, 41.903},
	{"milano", "Milano", "it", 9.19, 45.464},
	{"lisboa", "Lisboa", "pt", -9.139, 38.722},
	{"athens", "Αθήνα", "el", 23.728, 37.984},
	{"warszawa", "Warszawa", "pl", 21.012, 52.23},
	{"amsterdam", "Amsterdam", "nl", 4.895, 52.37},
	{"brussels", "Bruxelles", "fr", 4.352, 50.847},
	{"munich", "München", "de", 11.576, 48.137},
	{"berlin", "Berlin", "de", 13.4, 52.52},
	{"praha", "Praha", "cs", 14.421, 50.088},
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
		"Climate shelters and cool places in %s — ClimateUmbral",
		"Too hot in %s?",
		"Find an official climate shelter or an air-conditioned " +
			"public place near you — free, no app, no account.",
		"Find a cool place now",
		"Open the map",
		"%d official climate shelters published by the city are " +
			"on the map.",
		"Your city has not published a climate-shelter list yet — " +
			"the map still shows air-conditioned public places from " +
			"OpenStreetMap, and heat advice in your language.",
		"Drink water often. Wet your skin. Avoid the sun from " +
			"12:00 to 17:00. Dizzy or confused? Call 112.",
	},
	"es": {
		"Refugios climáticos y lugares frescos en %s — ClimateUmbral",
		"¿Demasiado calor en %s?",
		"Encuentra un refugio climático oficial o un lugar público " +
			"climatizado cerca de ti — gratis, sin app, sin cuenta.",
		"Buscar un lugar fresco ahora",
		"Abrir el mapa",
		"%d refugios climáticos oficiales publicados por la ciudad " +
			"están en el mapa.",
		"Tu ciudad aún no publica una lista de refugios " +
			"climáticos — el mapa muestra igualmente lugares " +
			"públicos climatizados de OpenStreetMap y consejos " +
			"contra el calor.",
		"Bebe agua a menudo. Mójate la piel. Evita el sol de " +
			"12:00 a 17:00. ¿Mareo o confusión? Llama al 112.",
	},
	"ca": {
		"Refugis climàtics i llocs frescos a %s — ClimateUmbral",
		"Massa calor a %s?",
		"Troba un refugi climàtic oficial o un lloc públic " +
			"climatitzat a prop teu — gratuït, sense app, sense compte.",
		"Troba un lloc fresc ara",
		"Obre el mapa",
		"%d refugis climàtics oficials publicats per la ciutat " +
			"són al mapa.",
		"La teva ciutat encara no publica una llista de refugis " +
			"climàtics — el mapa mostra igualment llocs públics " +
			"climatitzats d'OpenStreetMap i consells contra la calor.",
		"Beu aigua sovint. Mulla't la pell. Evita el sol de 12:00 " +
			"a 17:00. Mareig o confusió? Truca al 112.",
	},
	"fr": {
		"Refuges climatiques et lieux frais à %s — ClimateUmbral",
		"Trop chaud à %s ?",
		"Trouvez un refuge climatique officiel ou un lieu public " +
			"climatisé près de chez vous — gratuit, sans " +
			"application, sans compte.",
		"Trouver un lieu frais maintenant",
		"Ouvrir la carte",
		"%d refuges climatiques officiels publiés par la ville " +
			"sont sur la carte.",
		"Votre ville ne publie pas encore de liste de refuges — " +
			"la carte montre quand même les lieux publics " +
			"climatisés d'OpenStreetMap, et des conseils contre " +
			"la chaleur.",
		"Buvez de l'eau souvent. Mouillez votre peau. Évitez le " +
			"soleil de 12 h à 17 h. Vertiges, confusion ? " +
			"Appelez le 112.",
	},
	"de": {
		"Kühle Orte und Klima-Schutzräume in %s — ClimateUmbral",
		"Zu heiß in %s?",
		"Finden Sie einen offiziellen kühlen Ort oder einen " +
			"klimatisierten öffentlichen Raum in Ihrer Nähe — " +
			"kostenlos, ohne App, ohne Konto.",
		"Jetzt kühlen Ort finden",
		"Karte öffnen",
		"%d offizielle kühle Orte der Stadt sind auf der Karte.",
		"Ihre Stadt veröffentlicht noch keine Liste kühler Orte — " +
			"die Karte zeigt trotzdem klimatisierte öffentliche " +
			"Orte aus OpenStreetMap und Hitzetipps.",
		"Trinken Sie oft Wasser. Machen Sie die Haut nass. Meiden " +
			"Sie die Sonne von 12 bis 17 Uhr. Schwindel oder " +
			"Verwirrung? Rufen Sie 112 an.",
	},
	"it": {
		"Rifugi climatici e luoghi freschi a %s — ClimateUmbral",
		"Troppo caldo a %s?",
		"Trova un rifugio climatico ufficiale o un luogo pubblico " +
			"climatizzato vicino a te — gratis, senza app, senza " +
			"account.",
		"Trova subito un luogo fresco",
		"Apri la mappa",
		"%d rifugi climatici ufficiali pubblicati dalla città " +
			"sono sulla mappa.",
		"La tua città non pubblica ancora una lista di rifugi " +
			"climatici — la mappa mostra comunque i luoghi " +
			"pubblici climatizzati da OpenStreetMap e consigli " +
			"contro il caldo.",
		"Bevi acqua spesso. Bagnati la pelle. Evita il sole dalle " +
			"12:00 alle 17:00. Vertigini o confusione? Chiama il 112.",
	},
	"pt": {
		"Refúgios climáticos e lugares frescos em %s — ClimateUmbral",
		"Calor demais em %s?",
		"Encontre um refúgio climático oficial ou um lugar " +
			"público climatizado perto de si — grátis, sem " +
			"aplicação, sem conta.",
		"Encontrar um lugar fresco agora",
		"Abrir o mapa",
		"%d refúgios climáticos oficiais publicados pela cidade " +
			"estão no mapa.",
		"A sua cidade ainda não publica uma lista de refúgios " +
			"climáticos — o mapa mostra na mesma lugares públicos " +
			"climatizados do OpenStreetMap e conselhos contra o calor.",
		"Beba água muitas vezes. Molhe a pele. Evite o sol das " +
			"12:00 às 17:00. Tonturas ou confusão? Ligue 112.",
	},
	"nl": {
		"Koele plekken en klimaatschuilplaatsen in %s — ClimateUmbral",
		"Te heet in %s?",
		"Vind een officiële koele plek of een openbaar gebouw met " +
			"airco bij jou in de buurt — gratis, zonder app, " +
			"zonder account.",
		"Vind nu een koele plek",
		"Open de kaart",
		"%d officiële koele plekken van de stad staan op de kaart.",
		"Je stad publiceert nog geen lijst met koele plekken — de " +
			"kaart toont wel openbare plekken met airco uit " +
			"OpenStreetMap en tips tegen de hitte.",
		"Drink vaak water. Maak je huid nat. Vermijd de zon van " +
			"12:00 tot 17:00. Duizelig of verward? Bel 112.",
	},
	"pl": {
		"Miejsca chłodu w %s — ClimateUmbral",
		"Za gorąco w %s?",
		"Znajdź oficjalne miejsce chłodu lub klimatyzowane " +
			"miejsce publiczne w pobliżu — za darmo, bez " +
			"aplikacji, bez konta.",
		"Znajdź chłodne miejsce teraz",
		"Otwórz mapę",
		"%d oficjalnych miejsc chłodu opublikowanych przez miasto " +
			"jest na mapie.",
		"Twoje miasto nie opublikowało jeszcze listy miejsc " +
			"chłodu — mapa i tak pokazuje klimatyzowane miejsca " +
			"publiczne z OpenStreetMap i porady na upał.",
		"Pij często wodę. Zwilżaj skórę. Unikaj słońca od 12:00 " +
			"do 17:00. Zawroty głowy, dezorientacja? Dzwoń pod 112.",
	},
	"cs": {
		"Chladná místa v %s — ClimateUmbral",
		"Příliš horko v %s?",
		"Najděte oficiální chladné místo nebo klimatizované " +
			"veřejné místo ve svém okolí — zdarma, bez aplikace, " +
			"bez účtu.",
		"Najít chladné místo teď",
		"Otevřít mapu",
		"%d oficiálních chladných míst zveřejněných městem je " +
			"na mapě.",
		"Vaše město zatím seznam chladných míst nezveřejnilo — " +
			"mapa přesto ukazuje klimatizovaná veřejná místa z " +
			"OpenStreetMap a rady proti horku.",
		"Pijte často vodu. Namočte si pokožku. Vyhýbejte se " +
			"slunci od 12:00 do 17:00. Závratě nebo zmatenost? " +
			"Volejte 112.",
	},
	"el": {
		"Κλιματικά καταφύγια και δροσερά μέρη — %s — ClimateUmbral",
		"Πολλή ζέστη στην %s;",
		"Βρείτε ένα επίσημο κλιματικό καταφύγιο ή έναν " +
			"κλιματιζόμενο δημόσιο χώρο κοντά σας — δωρεάν, χωρίς " +
			"εφαρμογή, χωρίς λογαριασμό.",
		"Βρείτε δροσερό μέρος τώρα",
		"Ανοίξτε τον χάρτη",
		"%d επίσημα κλιματικά καταφύγια της πόλης είναι στον χάρτη.",
		"Η πόλη σας δεν έχει δημοσιεύσει ακόμη λίστα καταφυγίων — " +
			"ο χάρτης δείχνει κλιματιζόμενους δημόσιους χώρους από " +
			"το OpenStreetMap και συμβουλές για τον καύσωνα.",
		"Πίνετε νερό συχνά. Βρέξτε το δέρμα σας. Αποφύγετε τον " +
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
<a class="btn map" href="/#map">{{.CtaMap}}</a>
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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = cityTmpl.Execute(w, map[string]any{
			"Lang":         c.Lang,
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
