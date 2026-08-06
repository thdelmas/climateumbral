// Read-back for one-tap crowd verification: a tap must change what
// the next person sees, or reporting is a black hole. Counts come
// from /api/reports (public, whole numbers, no identity), keyed
// "target|issue" — folded here to target -> { issue: n }. The
// endpoint serves max-age=300; this cache just matches it so one
// page visit is one fetch.
const TTL = 5 * 60 * 1000
let cache = null
let fetchedAt = 0

export async function reportCounts() {
  const now = Date.now()
  if (cache && now - fetchedAt < TTL) return cache
  try {
    const res = await fetch('/api/reports')
    if (!res.ok) throw new Error(String(res.status))
    const flat = await res.json()
    const byTarget = {}
    for (const [k, n] of Object.entries(flat)) {
      const i = k.lastIndexOf('|')
      if (i < 0 || typeof n !== 'number') continue
      const target = k.slice(0, i)
      ;(byTarget[target] ??= {})[k.slice(i + 1)] = n
    }
    cache = byTarget
    fetchedAt = now
  } catch {
    // an unreachable ledger shows nothing — never a stale error
    cache = cache ?? {}
  }
  return cache
}

// Fixed display order: problems first, praise last.
export const REPORT_ISSUES = ['closed', 'hours', 'location', 'good']
