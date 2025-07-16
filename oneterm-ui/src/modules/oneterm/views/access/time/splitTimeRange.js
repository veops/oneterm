// Convert time string to minutes
function timeToMinutes(str) {
  const [h, m] = str.split(':').map(Number)
  return h * 60 + m
}

// Convert minutes to time string
function minutesToTime(mins) {
  const h = String(Math.floor(mins / 60)).padStart(2, '0')
  const m = String(mins % 60).padStart(2, '0')
  return `${h}:${m}`
}

// Split into half-hour intervals
function splitHalfHour(start, end) {
  const res = []
  let s = timeToMinutes(start)
  const e = timeToMinutes(end)
  while (s < e) {
    const next = Math.min(s + 30, e)
    res.push(`${minutesToTime(s)}~${minutesToTime(next)}`)
    s = next
  }
  return res
}

/**
 * split time range
 * @param {*} mergeData Array<{ weekdays: number[], start_time: 'HH:mm', end_time: 'HH:mm' }>
 * @returns Array<{ day: number, value: Array<'HH:mm'> }>
 */
export function splitTimeRange(timeRanges) {
  const weekMap = {}

  timeRanges.forEach(item => {
    item.weekdays.forEach(weekNum => {
      if (!weekMap[weekNum]) weekMap[weekNum] = []
      weekMap[weekNum].push(...splitHalfHour(item.start_time, item.end_time))
    })
  })

  // Remove duplicates and sort
  return Object.keys(weekMap)
    .sort((a, b) => a - b)
    .map(weekNum => {
      // Remove duplicates
      const valueSet = new Set(weekMap[weekNum])
      // Sort
      const value = Array.from(valueSet).sort((a, b) => {
        const [aStart] = a.split('~')
        const [bStart] = b.split('~')
        return timeToMinutes(aStart) - timeToMinutes(bStart)
      })
      return {
        day: Number(weekNum),
        value
      }
    })
}
