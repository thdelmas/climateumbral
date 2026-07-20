// Local identity: a pseudonym and the bearer tokens for acts made in
// this browser. Tokens are the only proof of authorship (no accounts),
// so losing this storage means losing the ability to flip/erase them.
const KEY = 'tilewhip' // storage key predates the ClimateUmbral rename; changing it orphans stored act tokens

function read() {
  try {
    return JSON.parse(localStorage.getItem(KEY)) ?? {}
  } catch {
    return {}
  }
}

function write(state) {
  localStorage.setItem(KEY, JSON.stringify(state))
}

export function myName() {
  return read().name ?? ''
}

export function setMyName(name) {
  write({ ...read(), name })
}

export function tokenFor(kind, x, y) {
  return read()[kind]?.[`${x},${y}`]
}

export function rememberToken(kind, x, y, token) {
  const state = read()
  state[kind] = { ...state[kind], [`${x},${y}`]: token }
  write(state)
}

export function forgetToken(kind, x, y) {
  const state = read()
  if (state[kind]) delete state[kind][`${x},${y}`]
  write(state)
}

export function allTokens(kind) {
  return { ...(read()[kind] ?? {}) }
}

export function openedTotal() {
  return read().opened ?? 0
}

export function addOpened(n) {
  const state = read()
  state.opened = (state.opened ?? 0) + n
  write(state)
}
