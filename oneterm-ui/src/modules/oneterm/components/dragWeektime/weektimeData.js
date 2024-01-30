const formatDate = (date, fmt) => {
    const o = {
        'M+': date.getMonth() + 1,
        'd+': date.getDate(),
        'h+': date.getHours(),
        'm+': date.getMinutes(),
        's+': date.getSeconds(),
        'q+': Math.floor((date.getMonth() + 3) / 3),
        S: date.getMilliseconds()
    }
    if (/(y+)/.test(fmt)) {
        fmt = fmt.replace(
            RegExp.$1,
            (date.getFullYear() + '').substr(4 - RegExp.$1.length)
        )
    }
    for (const k in o) {
        if (new RegExp('(' + k + ')').test(fmt)) {
            fmt = fmt.replace(
                RegExp.$1,
                RegExp.$1.length === 1 ? o[k] : ('00' + o[k]).substr(('' + o[k]).length)
            )
        }
    }
    return fmt
}

const createArr = (len) => {
    return Array.from(Array(len)).map((ret, id) => id)
}

const formatWeektime = (col) => {
    const timestamp = 1542384000000 // '2018-11-17 00:00:00'
    const beginstamp = timestamp + col * 1800000 // col * 30 * 60 * 1000
    const endstamp = beginstamp + 1800000

    const begin = formatDate(new Date(beginstamp), 'hh:mm')
    const end = formatDate(new Date(endstamp), 'hh:mm')
    return `${begin}~${end}`
}

const data = [
    '星期一',
    '星期二',
    '星期三',
    '星期四',
    '星期五',
    '星期六',
    '星期日'
].map((ret, index) => {
    const children = (ret, row, max) => {
        return createArr(max).map((t, col) => {
            return {
                week: ret,
                value: formatWeektime(col),
                begin: formatWeektime(col).split('~')[0],
                end: formatWeektime(col).split('~')[1],
                row,
                col
            }
        })
    }
    return {
        week: ret,
        row: index,
        child: children(ret, index, 48)
    }
})

export default data
