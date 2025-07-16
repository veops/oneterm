/**
 * merge time range
 * @param {*} timeRanges Array<{ day: number, value: Array<'HH:mm'> }>
 * @returns Array<{ weekdays: number[], start_time: 'HH:mm', end_time: 'HH:mm' }>
 */
export function mergeTimeRange(timeRanges) {
  // 1. Count which weekdays each interval appears on
  const intervalMap = {} // key: 'start~end', value: Set(day)
  timeRanges.forEach(item => {
    item.value.forEach(interval => {
      if (!intervalMap[interval]) intervalMap[interval] = new Set()
      intervalMap[interval].add(item.day)
    })
  })

  // 2. Group by interval and sort by weekday
  const intervalArr = Object.entries(intervalMap).map(([interval, weekSet]) => {
    const [start_time, end_time] = interval.split('~')
    return {
      start_time,
      end_time,
      weekdays: Array.from(weekSet).sort((a, b) => a - b)
    }
  })

  // 3. Merge consecutive intervals with exactly the same weekdays
  intervalArr.sort((a, b) => {
    // First by weekdays, then by start_time
    const w1 = a.weekdays.join(',')
    const w2 = b.weekdays.join(',')
    if (w1 !== w2) return w1.localeCompare(w2)
    return a.start_time.localeCompare(b.start_time)
  })

  const result = []
  for (let i = 0; i < intervalArr.length; i++) {
    const cur = intervalArr[i]
    if (
      result.length &&
      // Same weekdays as previous, and previous end_time equals current start_time
      JSON.stringify(result[result.length - 1].weekdays) === JSON.stringify(cur.weekdays) &&
      result[result.length - 1].end_time === cur.start_time
    ) {
      // Merge
      result[result.length - 1].end_time = cur.end_time
    } else {
      result.push({ ...cur })
    }
  }
  return result
}
