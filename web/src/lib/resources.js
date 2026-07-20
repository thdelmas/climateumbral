// Curated resources — the reading list behind the instrument.
// Every entry was verified live before landing here (link resolves,
// author and claim match); curation happens in git, so recommending
// one is a PR/issue, reviewed like code. Keep blurbs to one line of
// why-it-matters, not abstracts. kind: study | video | act.
export const RESOURCES = [
  {
    kind: 'study',
    title: 'Heat-related mortality in Europe during the summer ' +
      'of 2022',
    source: 'Ballester et al., Nature Medicine',
    year: 2023,
    url: 'https://www.nature.com/articles/s41591-023-02419-z',
    note: 'The scale of the problem: ~61,700 heat deaths in one ' +
      'European summer, counted country by country.',
  },
  {
    kind: 'study',
    title: 'Cooling cities through urban green infrastructure: a ' +
      'health impact assessment of European cities',
    source: 'Iungman et al., The Lancet',
    year: 2023,
    url: 'https://www.thelancet.com/journals/lancet/article/' +
      'PIIS0140-6736(22)02585-5/abstract',
    note: 'Over 4% of summer deaths in 93 EU cities trace to the ' +
      'urban heat island; 30% tree cover could prevent a third ' +
      'of them. The study this instrument’s levers lean on.',
  },
  {
    kind: 'study',
    title: 'Green space and mortality in European cities: a ' +
      'health impact assessment study',
    source: 'Barboza, Cirach, Khomenko et al., ' +
      'The Lancet Planetary Health',
    year: 2021,
    url: 'https://www.thelancet.com/journals/lanplh/article/' +
      'PIIS2542-5196(21)00229-1/fulltext',
    note: 'What meeting the WHO green-space recommendation would ' +
      'save, city by city — including yours.',
  },
  {
    kind: 'study',
    title: 'Heat-related mortality in Europe during 2023 and the ' +
      'role of adaptation in protecting health',
    source: 'Ballester et al., Nature Medicine',
    year: 2024,
    url: 'https://www.nature.com/articles/s41591-024-03186-1',
    note: 'The hopeful counterpart: adaptation measurably cuts ' +
      'deaths at the same temperatures. Acting works.',
  },
  {
    kind: 'video',
    title: 'How America’s hottest city is trying to cool down',
    source: 'Vox',
    year: 2021,
    url: 'https://www.youtube.com/watch?v=ZQ6fSHr5TJg',
    yt: 'ZQ6fSHr5TJg',
    note: 'Thermal cameras over Phoenix: why tree shade is ' +
      'infrastructure, and why the hottest blocks are the ' +
      'poorest ones.',
  },
  {
    kind: 'video',
    title: 'Can Our Cities Survive the Heat?',
    source: 'PBS Terra — Weathered',
    year: 2023,
    url: 'https://www.youtube.com/watch?v=qtinSxbRJV8',
    yt: 'qtinSxbRJV8',
    note: 'Portland’s heat dome, Medellín’s green ' +
      'corridors, Phoenix’s response — the whole arc in one ' +
      'episode.',
  },
  {
    kind: 'video',
    title: 'When Will Extreme Heat Become Unlivable?',
    source: 'PBS Terra — Weathered',
    year: 2024,
    url: 'https://www.pbs.org/video/' +
      'when-will-extreme-heat-become-unlivable-crd0jh/',
    yt: '7hBMbQ9de1g',
    note: 'The threshold question this map is built around, taken ' +
      'seriously: wet-bulb limits and who hits them first.',
  },
  {
    kind: 'video',
    title: 'FREE Water was ILLEGAL… He Changed That…',
    source: 'Andrew Millison',
    year: 2026,
    url: 'https://www.youtube.com/watch?v=lQ-jdUGCxcc',
    yt: 'lQ-jdUGCxcc',
    note: 'Brad Lancaster’s Tucson: cut the curb, harvest the ' +
      'street’s rain, shade the block with trees — one neighbor ' +
      'cooling a desert neighborhood by hand. The tree-pit lever, ' +
      'lived for thirty years.',
  },
  {
    kind: 'act',
    title: 'NK Tegelwippen',
    source: 'The Netherlands',
    year: 2020,
    url: 'https://www.nk-tegelwippen.nl/',
    note: 'The national tile-flipping championship that proved ' +
      'the mechanic: ~17 million garden tiles turned green. This ' +
      'project’s founding inspiration.',
  },
  {
    kind: 'act',
    title: 'Depave',
    source: 'Portland, Oregon',
    year: 2008,
    url: 'https://depave.org/',
    note: 'The organization that turned pavement removal into a ' +
      'community practice — how-to guides included.',
  },
  {
    kind: 'act',
    title: 'Climate-ADAPT',
    source: 'European Environment Agency',
    year: 2012,
    url: 'https://climate-adapt.eea.europa.eu/',
    note: 'The EU’s adaptation knowledge base: case studies, ' +
      'city plans and the evidence behind them, searchable.',
  },
  {
    kind: 'map',
    title: 'EXTREMA',
    source: 'Athens, Paris, Rotterdam, Milan, Mallorca, London',
    year: 2018,
    url: 'https://www.extrema-global.com/site-extrema-global/',
    note: 'A heat-risk app that guides you to the nearest cooling ' +
      'place, personalized to your health — city by partner city. ' +
      'If you live in one, install it.',
  },
  {
    kind: 'map',
    title: 'Berliner Erfrischungskarte',
    source: 'ODIS / Technologiestiftung Berlin',
    year: 2021,
    url: 'https://erfrischungskarte.odis-berlin.de/',
    note: 'Cool, shady and windy places in Berlin — with shade ' +
      'modeled for every hour of the day from a LIDAR surface ' +
      'model. The best hour-by-hour UX in this space.',
  },
  {
    kind: 'map',
    title: 'Cool Walks',
    source: 'Barcelona Regional',
    year: 2021,
    url: 'https://cool.bcnregional.com/',
    note: 'Shade-optimal walking routes — shortest, shadiest, or ' +
      'full "vampire mode" past drinking fountains. The routing ' +
      'idea every hot city will eventually need.',
  },
]

export const RESOURCE_KINDS = {
  study: { icon: '📄', label: 'The evidence' },
  video: { icon: '🎬', label: 'Watch' },
  act: { icon: '🛠️', label: 'Act' },
  map: { icon: '🗺️', label: 'Neighbor maps' },
}
