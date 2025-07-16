// Format a Date object to 'HH:mm'
const formatTime = (date) => {
  const h = String(date.getHours()).padStart(2, '0')
  const m = String(date.getMinutes()).padStart(2, '0')
  return `${h}:${m}`
}

// Generate half-hour intervals for one day, last end is 23:59
const generateDayIntervals = (day, row, total = 48) => {
  const intervals = []
  for (let i = 0; i < total; i++) {
    const startMinutes = i * 30
    const endMinutes = (i + 1) * 30
    const begin = formatTime(new Date(2000, 0, 1, 0, startMinutes))
    // Last interval ends at 23:59, others at next half hour
    const end =
      i === total - 1
        ? '23:59'
        : formatTime(new Date(2000, 0, 1, 0, endMinutes))
    intervals.push({
      day,
      value: `${begin}~${end}`,
      begin,
      end,
      row,
      col: i
    })
  }
  return intervals
}

// Generate week data: 7 days, each with 48 half-hour intervals
const weekTimeData = Array.from({ length: 7 }, (_, i) => ({
  day: i + 1,
  row: i,
  child: generateDayIntervals(i + 1, i, 48)
}))

export default weekTimeData
