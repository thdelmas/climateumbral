// Strings for the panic path (CoolNow + its doorway in App).
// Five languages: the four network cities' own plus English. Keys
// are stable; a missing language falls back to English.
//
// Vocabulary discipline: "official climate shelter" is a city's
// promise; "other cool public places" are crowd knowledge (OSM) —
// the two tiers are named differently everywhere they appear, so a
// mall can never borrow a city's authority.
export const STRINGS = {
  en: {
    title: 'Too hot?',
    find: 'Find a cool place near me',
    orCity: 'Or tap your city:',
    locating: 'Finding where you are…',
    nearOfficial: 'Official climate shelters',
    officialNote: 'Rooms promised by your city.',
    nearOther: 'Other cool public places',
    otherNote: 'Air-conditioned places open to the public (shops, ' +
      'cafés, malls…) — reported by people on OpenStreetMap, ' +
      'not checked by any city.',
    route: 'Show me the way',
    hours: 'Check opening hours before you go.',
    walkMin: 'min walk',
    openNow: 'Open now',
    closedNow: 'Closed now',
    until: 'until',
    todayW: 'today',
    closedToday: 'Closed today',
    advice: 'Against the heat',
    tips: [
      'Drink water often, even if not thirsty.',
      'Wet your skin, hair and clothes.',
      'Stay out of the sun from 12:00 to 17:00.',
      'Dizzy, confused, or feeling faint?',
    ],
    call: 'Call 112 now',
    locDenied: 'Location is blocked for this site. Allow it in ' +
      'your phone settings — or tap your city:',
    locFail: 'Could not find you yet. Try once more — or tap ' +
      'your city:',
    noData: 'The shelter list is not reachable right now. ' +
      'The advice below still helps.',
    far: 'No official climate shelter published near you — your ' +
      'city has not shared its list yet.',
    farNearest: 'Nearest known official shelter:',
    back: 'Back to the map',
  },
  fr: {
    title: 'Trop chaud ?',
    find: 'Trouver un lieu frais près de moi',
    orCity: 'Ou touchez votre ville :',
    locating: 'Recherche de votre position…',
    nearOfficial: 'Refuges climatiques officiels',
    officialNote: 'Des salles promises par votre ville.',
    nearOther: 'Autres lieux publics frais',
    otherNote: 'Lieux climatisés ouverts au public (commerces, ' +
      'cafés, centres commerciaux…) — signalés par des gens sur ' +
      'OpenStreetMap, non vérifiés par une ville.',
    route: 'Montrez-moi le chemin',
    hours: 'Vérifiez les horaires avant d’y aller.',
    walkMin: 'min à pied',
    openNow: 'Ouvert maintenant',
    closedNow: 'Fermé maintenant',
    until: 'jusqu’à',
    todayW: 'aujourd’hui',
    closedToday: 'Fermé aujourd’hui',
    advice: 'Contre la chaleur',
    tips: [
      'Buvez de l’eau souvent, même sans soif.',
      'Mouillez votre peau, vos cheveux, vos vêtements.',
      'Évitez le soleil entre 12 h et 17 h.',
      'Vertiges, confusion, malaise ?',
    ],
    call: 'Appelez le 112',
    locDenied: 'La localisation est bloquée pour ce site. ' +
      'Autorisez-la dans les réglages — ou touchez votre ville :',
    locFail: 'Position introuvable pour l’instant. Réessayez — ' +
      'ou touchez votre ville :',
    noData: 'La liste des refuges est indisponible pour le moment. ' +
      'Les conseils ci-dessous restent valables.',
    far: 'Aucun refuge climatique officiel publié près de vous — ' +
      'votre ville n’a pas encore partagé sa liste.',
    farNearest: 'Refuge officiel connu le plus proche :',
    back: 'Retour à la carte',
  },
  es: {
    title: '¿Demasiado calor?',
    find: 'Buscar un lugar fresco cerca de mí',
    orCity: 'O toca tu ciudad:',
    locating: 'Buscando tu posición…',
    nearOfficial: 'Refugios climáticos oficiales',
    officialNote: 'Salas prometidas por tu ciudad.',
    nearOther: 'Otros lugares públicos frescos',
    otherNote: 'Lugares climatizados abiertos al público ' +
      '(tiendas, cafés, centros comerciales…) — señalados por ' +
      'gente en OpenStreetMap, no verificados por ninguna ciudad.',
    route: 'Enséñame el camino',
    hours: 'Comprueba los horarios antes de ir.',
    walkMin: 'min a pie',
    openNow: 'Abierto ahora',
    closedNow: 'Cerrado ahora',
    until: 'hasta las',
    todayW: 'hoy',
    closedToday: 'Cerrado hoy',
    advice: 'Contra el calor',
    tips: [
      'Bebe agua a menudo, aunque no tengas sed.',
      'Mójate la piel, el pelo y la ropa.',
      'Evita el sol de 12:00 a 17:00.',
      '¿Mareo, confusión, desmayo?',
    ],
    call: 'Llama al 112',
    locDenied: 'La ubicación está bloqueada para este sitio. ' +
      'Permítela en los ajustes — o toca tu ciudad:',
    locFail: 'Aún no se pudo encontrar tu posición. Prueba otra ' +
      'vez — o toca tu ciudad:',
    noData: 'La lista de refugios no está disponible ahora. ' +
      'Los consejos de abajo siguen sirviendo.',
    far: 'No hay refugios climáticos oficiales publicados cerca — ' +
      'tu ciudad aún no ha compartido su lista.',
    farNearest: 'Refugio oficial conocido más cercano:',
    back: 'Volver al mapa',
  },
  ca: {
    title: 'Massa calor?',
    find: 'Troba un lloc fresc a prop meu',
    orCity: 'O toca la teva ciutat:',
    locating: 'Cercant la teva posició…',
    nearOfficial: 'Refugis climàtics oficials',
    officialNote: 'Sales promeses per la teva ciutat.',
    nearOther: 'Altres llocs públics frescos',
    otherNote: 'Llocs climatitzats oberts al públic (botigues, ' +
      'cafès, centres comercials…) — assenyalats per gent a ' +
      'OpenStreetMap, no verificats per cap ciutat.',
    route: 'Ensenya’m el camí',
    hours: 'Comprova els horaris abans d’anar-hi.',
    walkMin: 'min a peu',
    openNow: 'Obert ara',
    closedNow: 'Tancat ara',
    until: 'fins a les',
    todayW: 'avui',
    closedToday: 'Tancat avui',
    advice: 'Contra la calor',
    tips: [
      'Beu aigua sovint, encara que no tinguis set.',
      'Mulla’t la pell, els cabells i la roba.',
      'Evita el sol de 12:00 a 17:00.',
      'Mareig, confusió, desmai?',
    ],
    call: 'Truca al 112',
    locDenied: 'La ubicació està bloquejada per a aquest lloc. ' +
      'Permet-la als ajustos — o toca la teva ciutat:',
    locFail: 'Encara no s’ha trobat la teva posició. Torna-ho a ' +
      'provar — o toca la teva ciutat:',
    noData: 'La llista de refugis no està disponible ara. ' +
      'Els consells d’aquí sota continuen servint.',
    far: 'No hi ha refugis climàtics oficials publicats a prop — ' +
      'la teva ciutat encara no ha compartit la llista.',
    farNearest: 'Refugi oficial conegut més proper:',
    back: 'Tornar al mapa',
  },
  de: {
    title: 'Zu heiß?',
    find: 'Kühlen Ort in meiner Nähe finden',
    orCity: 'Oder Stadt antippen:',
    locating: 'Standort wird gesucht…',
    nearOfficial: 'Offizielle kühle Orte',
    officialNote: 'Räume, die Ihre Stadt verspricht.',
    nearOther: 'Weitere kühle öffentliche Orte',
    otherNote: 'Klimatisierte, öffentlich zugängliche Orte ' +
      '(Geschäfte, Cafés, Einkaufszentren…) — von Menschen auf ' +
      'OpenStreetMap gemeldet, von keiner Stadt geprüft.',
    route: 'Zeig mir den Weg',
    hours: 'Öffnungszeiten vorher prüfen.',
    walkMin: 'Min. zu Fuß',
    openNow: 'Jetzt geöffnet',
    closedNow: 'Jetzt geschlossen',
    until: 'bis',
    todayW: 'heute',
    closedToday: 'Heute geschlossen',
    advice: 'Gegen die Hitze',
    tips: [
      'Trinken Sie oft Wasser, auch ohne Durst.',
      'Machen Sie Haut, Haare und Kleidung nass.',
      'Meiden Sie die Sonne von 12 bis 17 Uhr.',
      'Schwindel, Verwirrung, Schwäche?',
    ],
    call: '112 anrufen',
    locDenied: 'Der Standort ist für diese Seite blockiert. In ' +
      'den Einstellungen erlauben — oder Stadt antippen:',
    locFail: 'Standort noch nicht gefunden. Noch einmal ' +
      'versuchen — oder Stadt antippen:',
    noData: 'Die Liste ist gerade nicht erreichbar. ' +
      'Die Tipps unten helfen trotzdem.',
    far: 'Kein offizieller kühler Ort in Ihrer Nähe ' +
      'veröffentlicht — Ihre Stadt teilt ihre Liste noch nicht.',
    farNearest: 'Nächster bekannter offizieller Ort:',
    back: 'Zurück zur Karte',
  },
}

export const LANGS = Object.keys(STRINGS)

export function pickLang() {
  for (const l of navigator.languages ?? [navigator.language]) {
    const two = (l ?? '').slice(0, 2).toLowerCase()
    if (STRINGS[two]) return two
  }
  return 'en'
}
