// Strings for the panic path (CoolNow + its doorway in App).
// Five languages: the four network cities' own plus English. Keys
// are stable; a missing language falls back to English.
export const STRINGS = {
  en: {
    title: 'Too hot?',
    find: 'Find a cool room near me',
    orCity: 'Or tap your city:',
    locating: 'Finding where you are…',
    near: 'Cool rooms near you',
    route: 'Show me the way',
    hours: 'Check opening hours before you go.',
    walkMin: 'min walk',
    advice: 'Against the heat',
    tips: [
      'Drink water often, even if not thirsty.',
      'Wet your skin, hair and clothes.',
      'Stay out of the sun from 12:00 to 17:00.',
      'Dizzy, confused, or feeling faint?',
    ],
    call: 'Call 112 now',
    denied: 'Could not get your location. Tap your city:',
    noData: 'The shelter list is not reachable right now. ' +
      'The advice below still helps.',
    far: 'No published cool room near you — your city has not ' +
      'shared its list yet. The advice below helps.',
    farNearest: 'Nearest known cool room:',
    back: 'Back to the map',
  },
  fr: {
    title: 'Trop chaud ?',
    find: 'Trouver une salle fraîche près de moi',
    orCity: 'Ou touchez votre ville :',
    locating: 'Recherche de votre position…',
    near: 'Salles fraîches près de vous',
    route: 'Montrez-moi le chemin',
    hours: 'Vérifiez les horaires avant d’y aller.',
    walkMin: 'min à pied',
    advice: 'Contre la chaleur',
    tips: [
      'Buvez de l’eau souvent, même sans soif.',
      'Mouillez votre peau, vos cheveux, vos vêtements.',
      'Évitez le soleil entre 12 h et 17 h.',
      'Vertiges, confusion, malaise ?',
    ],
    call: 'Appelez le 112',
    denied: 'Position introuvable. Touchez votre ville :',
    noData: 'La liste des refuges est indisponible pour le moment. ' +
      'Les conseils ci-dessous restent valables.',
    far: 'Aucune salle fraîche publiée près de vous — votre ville ' +
      'n’a pas encore partagé sa liste. Les conseils ci-dessous aident.',
    farNearest: 'Salle fraîche connue la plus proche :',
    back: 'Retour à la carte',
  },
  es: {
    title: '¿Demasiado calor?',
    find: 'Buscar una sala fresca cerca de mí',
    orCity: 'O toca tu ciudad:',
    locating: 'Buscando tu posición…',
    near: 'Salas frescas cerca de ti',
    route: 'Enséñame el camino',
    hours: 'Comprueba los horarios antes de ir.',
    walkMin: 'min a pie',
    advice: 'Contra el calor',
    tips: [
      'Bebe agua a menudo, aunque no tengas sed.',
      'Mójate la piel, el pelo y la ropa.',
      'Evita el sol de 12:00 a 17:00.',
      '¿Mareo, confusión, desmayo?',
    ],
    call: 'Llama al 112',
    denied: 'No se pudo obtener tu posición. Toca tu ciudad:',
    noData: 'La lista de refugios no está disponible ahora. ' +
      'Los consejos de abajo siguen sirviendo.',
    far: 'No hay salas frescas publicadas cerca — tu ciudad aún no ' +
      'ha compartido su lista. Los consejos de abajo ayudan.',
    farNearest: 'Sala fresca conocida más cercana:',
    back: 'Volver al mapa',
  },
  ca: {
    title: 'Massa calor?',
    find: 'Troba una sala fresca a prop meu',
    orCity: 'O toca la teva ciutat:',
    locating: 'Cercant la teva posició…',
    near: 'Sales fresques a prop teu',
    route: 'Ensenya’m el camí',
    hours: 'Comprova els horaris abans d’anar-hi.',
    walkMin: 'min a peu',
    advice: 'Contra la calor',
    tips: [
      'Beu aigua sovint, encara que no tinguis set.',
      'Mulla’t la pell, els cabells i la roba.',
      'Evita el sol de 12:00 a 17:00.',
      'Mareig, confusió, desmai?',
    ],
    call: 'Truca al 112',
    denied: 'No s’ha pogut obtenir la posició. Toca la teva ciutat:',
    noData: 'La llista de refugis no està disponible ara. ' +
      'Els consells d’aquí sota continuen servint.',
    far: 'No hi ha sales fresques publicades a prop — la teva ciutat ' +
      'encara no ha compartit la llista. Els consells d’aquí sota ajuden.',
    farNearest: 'Sala fresca coneguda més propera:',
    back: 'Tornar al mapa',
  },
  de: {
    title: 'Zu heiß?',
    find: 'Kühlen Raum in meiner Nähe finden',
    orCity: 'Oder Stadt antippen:',
    locating: 'Standort wird gesucht…',
    near: 'Kühle Räume in Ihrer Nähe',
    route: 'Zeig mir den Weg',
    hours: 'Öffnungszeiten vorher prüfen.',
    walkMin: 'Min. zu Fuß',
    advice: 'Gegen die Hitze',
    tips: [
      'Trinken Sie oft Wasser, auch ohne Durst.',
      'Machen Sie Haut, Haare und Kleidung nass.',
      'Meiden Sie die Sonne von 12 bis 17 Uhr.',
      'Schwindel, Verwirrung, Schwäche?',
    ],
    call: '112 anrufen',
    denied: 'Standort nicht gefunden. Stadt antippen:',
    noData: 'Die Liste ist gerade nicht erreichbar. ' +
      'Die Tipps unten helfen trotzdem.',
    far: 'Kein veröffentlichter kühler Raum in der Nähe — Ihre Stadt ' +
      'teilt ihre Liste noch nicht. Die Tipps unten helfen.',
    farNearest: 'Nächster bekannter kühler Raum:',
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
